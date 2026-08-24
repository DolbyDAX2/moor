WAILS_TAGS := production,webkit2_41

.PHONY: build install test clean appimage

# Tek binary: CLI + masaüstü GUI (argümansız çalıştırma da GUI'yi açar).
build:
	go build -tags "$(WAILS_TAGS)" -o moor .

install: build
	install -d $(HOME)/.local/bin
	install moor $(HOME)/.local/bin/moor

test:
	go vet ./...
	go test ./...

# Bağımlılıkları paketlemiş, kurulum gerektirmeyen AppImage üretir.
# Gereksinim: /tmp/linuxdeploy.AppImage (github.com/linuxdeploy/linuxdeploy)
# ve webkit2gtk-4.1 (sistemde).
appimage: build
	rm -rf dist/AppDir
	mkdir -p dist/AppDir/usr/bin dist/AppDir/usr/share/applications \
	         dist/AppDir/usr/share/icons/hicolor/256x256/apps
	cp moor dist/AppDir/usr/bin/moor
	cp dist/moor.desktop dist/AppDir/usr/share/applications/moor.desktop
	cp dist/moor.png dist/AppDir/usr/share/icons/hicolor/256x256/apps/moor.png
	cd dist && /tmp/linuxdeploy.AppImage --appdir ./AppDir
	cd dist && /tmp/linuxdeploy.AppImage --appdir ./AppDir -o appimage

clean:
	rm -f moor
	rm -rf dist
