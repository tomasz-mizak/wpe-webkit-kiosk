IMAGE   := wpe-kiosk-builder
OUT     := output
VERSION ?= 0.0.0-dev

.PHONY: deb clean

deb:
	docker build -t $(IMAGE) .
	mkdir -p $(OUT)
	docker run --rm -e PKG_VERSION=$(VERSION) -v $(CURDIR)/src:/build/src/app:ro -v $(CURDIR)/cmd/kiosk:/build/src/cli:ro -v $(CURDIR)/debian:/build/debian:ro -v $(CURDIR)/extensions:/build/extensions:ro -v $(CURDIR)/$(OUT):/output $(IMAGE)

clean:
	rm -rf $(OUT)
