# Runs inside Docker — compiles kiosk launcher and packages .deb
WEBKIT_VER       := 2.50.5

PREFIX           := /opt/wpe-webkit-kiosk
ARCH             := $(shell dpkg-architecture -qDEB_HOST_MULTIARCH)
STAGING          := /build/staging
PKG_CONFIG_PATH  := $(PREFIX)/lib/$(ARCH)/pkgconfig:$(PREFIX)/lib/pkgconfig:$(PREFIX)/share/pkgconfig

export PKG_CONFIG_PATH

.PHONY: all package

all: launcher cli api package

# ---------- kiosk launcher ----------

launcher:
	gcc -O2 -Wall -Wextra -o $(STAGING)$(PREFIX)/bin/wpe-webkit-kiosk-bin /build/src/app/kiosk.c \
		$$(pkg-config --cflags --libs wpe-webkit-2.0 wpe-platform-2.0 json-glib-1.0)

# ---------- kiosk CLI ----------

cli:
	mkdir -p $(STAGING)/usr/bin
	cd /build/src/cli && CGO_ENABLED=0 go build -ldflags='-s -w' -o $(STAGING)/usr/bin/kiosk .

# ---------- kiosk API server ----------

api:
	mkdir -p $(STAGING)$(PREFIX)/bin
	cd /build/src/cli && CGO_ENABLED=0 go build -ldflags='-s -w' -o $(STAGING)$(PREFIX)/bin/kiosk-api ./cmd/api

# ---------- package ----------

package: launcher cli api
	cp /build/debian/wpe-webkit-kiosk $(STAGING)$(PREFIX)/bin/
	chmod +x $(STAGING)$(PREFIX)/bin/wpe-webkit-kiosk
	cp /build/debian/kiosk-start $(STAGING)$(PREFIX)/bin/
	chmod +x $(STAGING)$(PREFIX)/bin/kiosk-start
	mkdir -p $(STAGING)/etc/wpe-webkit-kiosk
	cp /build/debian/config $(STAGING)/etc/wpe-webkit-kiosk/
	mkdir -p $(STAGING)/usr/lib/systemd/system
	cp /build/debian/wpe-webkit-kiosk.service $(STAGING)/usr/lib/systemd/system/
	cp /build/debian/wpe-webkit-kiosk-vnc.service $(STAGING)/usr/lib/systemd/system/
	cp /build/debian/wpe-webkit-kiosk-api.service $(STAGING)/usr/lib/systemd/system/
	cp /build/debian/wpe-webkit-kiosk-vnc-check $(STAGING)$(PREFIX)/bin/
	chmod +x $(STAGING)$(PREFIX)/bin/wpe-webkit-kiosk-vnc-check
	mkdir -p $(STAGING)/usr/share/dbus-1/system.d
	cp /build/debian/com.wpe.Kiosk.conf $(STAGING)/usr/share/dbus-1/system.d/
	mkdir -p $(STAGING)/etc/sudoers.d
	cp /build/debian/sudoers $(STAGING)/etc/sudoers.d/wpe-webkit-kiosk
	chmod 440 $(STAGING)/etc/sudoers.d/wpe-webkit-kiosk
	mkdir -p $(STAGING)$(PREFIX)/extensions
	if [ -d /build/extensions ]; then cp -r /build/extensions/* $(STAGING)$(PREFIX)/extensions/; fi
	mkdir -p $(STAGING)/DEBIAN
	cp /build/debian/control $(STAGING)/DEBIAN/
	cp /build/debian/postinst $(STAGING)/DEBIAN/
	cp /build/debian/prerm $(STAGING)/DEBIAN/
	chmod 755 $(STAGING)/DEBIAN/postinst $(STAGING)/DEBIAN/prerm
	echo "/etc/wpe-webkit-kiosk/config" > $(STAGING)/DEBIAN/conffiles
	dpkg-deb --build $(STAGING) /output/wpe-webkit-kiosk_$(WEBKIT_VER)_amd64.deb
