# Runs inside Docker — builds WPE WebKit with WPEPlatform and kiosk launcher
LIBWPE_VER       := 1.16.3
WEBKIT_VER       := 2.50.5

PREFIX           := /opt/wpe-kiosk
JOBS             := $(shell nproc)
ARCH             := $(shell dpkg-architecture -qDEB_HOST_MULTIARCH)
SRC              := /build/src
STAGING          := /build/staging
PKG_CONFIG_PATH  := $(PREFIX)/lib/$(ARCH)/pkgconfig:$(PREFIX)/lib/pkgconfig:$(PREFIX)/share/pkgconfig

LIBWPE_TAR       := $(SRC)/libwpe-$(LIBWPE_VER).tar.xz
WEBKIT_TAR       := $(SRC)/wpewebkit-$(WEBKIT_VER).tar.xz
STAMP_LIBWPE     := $(SRC)/.stamp-libwpe
STAMP_WEBKIT     := $(SRC)/.stamp-webkit
STAMP_LAUNCHER   := $(SRC)/.stamp-launcher

export PKG_CONFIG_PATH

.PHONY: all download package

all: download $(STAMP_LIBWPE) $(STAMP_WEBKIT) $(STAMP_LAUNCHER) package

# ---------- download ----------

download: $(LIBWPE_TAR) $(WEBKIT_TAR)

$(LIBWPE_TAR):
	mkdir -p $(SRC)
	wget -qO $@ https://wpewebkit.org/releases/libwpe-$(LIBWPE_VER).tar.xz

$(WEBKIT_TAR):
	mkdir -p $(SRC)
	wget -qO $@ https://wpewebkit.org/releases/wpewebkit-$(WEBKIT_VER).tar.xz

# ---------- libwpe ----------

$(STAMP_LIBWPE): $(LIBWPE_TAR)
	cd $(SRC) && tar -xf libwpe-$(LIBWPE_VER).tar.xz
	meson setup $(SRC)/libwpe-$(LIBWPE_VER)/build $(SRC)/libwpe-$(LIBWPE_VER) \
		--buildtype=release --prefix=$(PREFIX)
	ninja -C $(SRC)/libwpe-$(LIBWPE_VER)/build -j $(JOBS)
	DESTDIR=$(STAGING) ninja -C $(SRC)/libwpe-$(LIBWPE_VER)/build install
	ninja -C $(SRC)/libwpe-$(LIBWPE_VER)/build install
	touch $@

# ---------- wpe-webkit (with WPEPlatform) ----------

$(STAMP_WEBKIT): $(STAMP_LIBWPE) $(WEBKIT_TAR)
	cd $(SRC) && tar -xf wpewebkit-$(WEBKIT_VER).tar.xz
	cmake -S $(SRC)/wpewebkit-$(WEBKIT_VER) -B $(SRC)/wpewebkit-$(WEBKIT_VER)/build -G Ninja \
		-DPORT=WPE \
		-DCMAKE_BUILD_TYPE=Release \
		-DCMAKE_INSTALL_PREFIX=$(PREFIX) \
		-DCMAKE_C_COMPILER=/usr/bin/clang-18 \
		-DCMAKE_CXX_COMPILER=/usr/bin/clang++-18 \
		-DENABLE_WPE_PLATFORM=ON \
		-DENABLE_WPE_PLATFORM_WAYLAND=ON \
		-DENABLE_WPE_PLATFORM_DRM=OFF \
		-DENABLE_WPE_PLATFORM_HEADLESS=OFF \
		-DENABLE_WPE_LEGACY_API=OFF \
		-DENABLE_DOCUMENTATION=OFF \
		-DENABLE_INTROSPECTION=OFF \
		-DENABLE_BUBBLEWRAP_SANDBOX=OFF \
		-DUSE_JPEGXL=OFF \
		-DUSE_LIBBACKTRACE=OFF \
		-DENABLE_SPEECH_SYNTHESIS=OFF
	ninja -C $(SRC)/wpewebkit-$(WEBKIT_VER)/build -j $(JOBS)
	DESTDIR=$(STAGING) ninja -C $(SRC)/wpewebkit-$(WEBKIT_VER)/build install
	ninja -C $(SRC)/wpewebkit-$(WEBKIT_VER)/build install
	touch $@

# ---------- kiosk launcher ----------

$(STAMP_LAUNCHER): $(STAMP_WEBKIT)
	gcc -O2 -Wall -Wextra -o $(STAGING)$(PREFIX)/bin/wpe-kiosk-bin /build/src/app/kiosk.c \
		$$(pkg-config --cflags --libs wpe-webkit-2.0 wpe-platform-2.0)
	touch $@

# ---------- package ----------

package: $(STAMP_LAUNCHER)
	cp /build/debian/wpe-kiosk $(STAGING)$(PREFIX)/bin/
	chmod +x $(STAGING)$(PREFIX)/bin/wpe-kiosk
	mkdir -p $(STAGING)/etc/wpe-kiosk
	cp /build/debian/config $(STAGING)/etc/wpe-kiosk/
	mkdir -p $(STAGING)/usr/lib/systemd/system
	cp /build/debian/wpe-kiosk.service $(STAGING)/usr/lib/systemd/system/
	mkdir -p $(STAGING)/usr/share/dbus-1/system.d
	cp /build/debian/com.wpe.Kiosk.conf $(STAGING)/usr/share/dbus-1/system.d/
	mkdir -p $(STAGING)/DEBIAN
	cp /build/debian/control $(STAGING)/DEBIAN/
	dpkg-deb --build $(STAGING) /output/wpe-kiosk_$(WEBKIT_VER)_amd64.deb
