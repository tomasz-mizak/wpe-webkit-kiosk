# Runs inside Docker — compiles kiosk launcher and packages .deb
WEBKIT_VER       := 2.50.5

PREFIX           := /opt/wpe-webkit-kiosk
ARCH             := $(shell dpkg-architecture -qDEB_HOST_MULTIARCH)
STAGING          := /build/staging
PKG_CONFIG_PATH  := $(PREFIX)/lib/$(ARCH)/pkgconfig:$(PREFIX)/lib/pkgconfig:$(PREFIX)/share/pkgconfig

export PKG_CONFIG_PATH

.PHONY: all package

all: launcher package

# ---------- kiosk launcher ----------

launcher:
	gcc -O2 -Wall -Wextra -o $(STAGING)$(PREFIX)/bin/wpe-webkit-kiosk-bin /build/src/app/kiosk.c \
		$$(pkg-config --cflags --libs wpe-webkit-2.0 wpe-platform-2.0)

# ---------- package ----------

package: launcher
	cp /build/debian/wpe-webkit-kiosk $(STAGING)$(PREFIX)/bin/
	chmod +x $(STAGING)$(PREFIX)/bin/wpe-webkit-kiosk
	mkdir -p $(STAGING)/etc/wpe-webkit-kiosk
	cp /build/debian/config $(STAGING)/etc/wpe-webkit-kiosk/
	mkdir -p $(STAGING)/usr/lib/systemd/system
	cp /build/debian/wpe-webkit-kiosk.service $(STAGING)/usr/lib/systemd/system/
	mkdir -p $(STAGING)/usr/share/dbus-1/system.d
	cp /build/debian/com.wpe.Kiosk.conf $(STAGING)/usr/share/dbus-1/system.d/
	mkdir -p $(STAGING)/DEBIAN
	cp /build/debian/control $(STAGING)/DEBIAN/
	cp /build/debian/postinst $(STAGING)/DEBIAN/
	cp /build/debian/prerm $(STAGING)/DEBIAN/
	chmod 755 $(STAGING)/DEBIAN/postinst $(STAGING)/DEBIAN/prerm
	dpkg-deb --build $(STAGING) /output/wpe-webkit-kiosk_$(WEBKIT_VER)_amd64.deb
