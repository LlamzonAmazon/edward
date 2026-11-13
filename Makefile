.PHONY: test mock app

WEBDIR := internal/server/web
APP := EDwaRd.app

test:
	go test ./...

mock:
	npm --prefix mock ci
	npm --prefix mock run build

app:
	@if [ ! -d mock/dist ]; then echo "mock/dist missing; run make mock first"; exit 1; fi
	mkdir -p $(WEBDIR)
	rm -rf $(WEBDIR)/*
	cp -R mock/dist/. $(WEBDIR)/
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources
	cp internal/pack/Info.plist $(APP)/Contents/Info.plist
	go build -o $(APP)/Contents/MacOS/edward ./cmd/edward
	codesign --force --deep -s - $(APP)
