WAILS_TAGS := production,webkit2_41

.PHONY: build build-win release-win install test clean appimage

# Tek binary: CLI + masaüstü GUI (argümansız çalıştırma da GUI'yi açar).
build:
	go build -tags "$(WAILS_TAGS)" -o moor .

# Windows derlemesi (cgo gerektirmez; GUI için WebView2 runtime gerekir).
# webkit2_41 etiketi Linux'a özgüdür, Windows'ta yalnız production kullanılır.
build-win:
	go build -tags production -ldflags "-H windowsgui" -o moor.exe .

# Windows üzerinde Inno Setup 6 ile kurulum EXE'si ve taşınabilir EXE üretir.
release-win:
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/build-windows-release.ps1

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
	# NO_STRIP: linuxdeploy'ın paketli eski strip'i yeni glibc kütüphanelerini
	# (.relr.dyn) okuyamıyor; sistem binutils zaten çok daha yeni.
	cd dist && NO_STRIP=1 /tmp/linuxdeploy.AppImage --appdir ./AppDir
	cd dist && NO_STRIP=1 /tmp/linuxdeploy.AppImage --appdir ./AppDir -o appimage

clean:
	rm -f moor moor.exe moor.exe~ moor_windows_amd64.syso
	rm -rf dist/AppDir dist/windows
	rm -f dist/*.AppImage
