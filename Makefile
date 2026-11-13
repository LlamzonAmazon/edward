.PHONY: test mock web app release

VERSION ?= 1.0
WEBDIR := internal/server/web
APP := EDwaRd.app
LDFLAGS := -s -w -X github.com/LlamzonAmazon/edward/internal/version.String=$(VERSION)

# bundle,<binary>,<app dir>: wrap a built binary in a stamped, ad-hoc signed app.
define bundle
	rm -rf $(2)
	mkdir -p $(2)/Contents/MacOS
	cp internal/pack/Info.plist $(2)/Contents/Info.plist
	plutil -replace CFBundleShortVersionString -string "$(VERSION)" $(2)/Contents/Info.plist
	cp $(1) $(2)/Contents/MacOS/edward
	codesign --force -s - $(2)
endef

test:
	go test ./...

mock:
	npm --prefix mock ci
	npm --prefix mock run build

web:
	@if [ ! -d mock/dist ]; then echo "mock/dist missing; run make mock first"; exit 1; fi
	mkdir -p $(WEBDIR)
	rm -rf $(WEBDIR)/*
	cp -R mock/dist/. $(WEBDIR)/

# app builds ./EDwaRd.app for this Mac. Then: sh install.sh
app: web
	mkdir -p .build
	rm -f .build/edward-native
	go build -trimpath -ldflags "$(LDFLAGS)" -o .build/edward-native ./cmd/edward
	$(call bundle,.build/edward-native,$(APP))

# release builds the universal tarball and checksum that install.sh downloads.
release: web
	rm -rf dist .build
	mkdir -p dist .build
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o .build/edward-arm64 ./cmd/edward
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o .build/edward-amd64 ./cmd/edward
	lipo -create -output .build/edward .build/edward-arm64 .build/edward-amd64
	$(call bundle,.build/edward,.build/$(APP))
	COPYFILE_DISABLE=1 tar -C .build -czf dist/edward-macos.tar.gz $(APP)
	cd dist && shasum -a 256 edward-macos.tar.gz > SHA256SUMS
	cat dist/SHA256SUMS
