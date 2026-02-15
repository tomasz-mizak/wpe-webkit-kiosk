IMAGE := wpe-kiosk-builder
OUT   := output

.PHONY: deb clean

deb:
	docker build -t $(IMAGE) .
	mkdir -p $(OUT)
	docker run --rm -v $(CURDIR)/src:/build/src/app:ro -v $(CURDIR)/cmd/kiosk:/build/src/cli:ro -v $(CURDIR)/debian:/build/debian:ro -v $(CURDIR)/$(OUT):/output $(IMAGE)

clean:
	rm -rf $(OUT)
