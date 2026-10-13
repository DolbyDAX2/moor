# Değişiklikler

## v0.1.2

### Windows
- CLI ve Wails masaüstü uygulaması için Windows desteği.
- Saf Go ConPTY katmanı; ayrı giriş/çıkış pipe'larıyla gömülü SSH terminali.
- OpenSSH istemcisi seçimi, boşluk içeren program yolları ve boyutlandırma düzeltmeleri.
- Terminal kapanırken son çıktının kaybolmasını önleyen okuma/çıkış koordinasyonu.
- Senkronlanmış özel anahtar içeriğinin Windows'ta kullanılabilmesi.
- Platforma uygun veri yolları ve dış terminal başlatma.
- Kullanıcı başına kurulum, kısayollar ve kaldırıcı içeren x64 kurulum EXE'si.
- WebView2 bootstrapper, EXE sürüm bilgisi, uygulama simgesi ve SHA-256 dosyası.

### Ortak
- GUI ve CLI'de giriş/kayıt sonrası otomatik bulut eşitlemesi.
- Windows ortam bloğundaki çift NUL hatası için regresyon testi.
- İsteğe bağlı gerçek SSH/ConPTY entegrasyon testi.
- Windows/Linux kurulum ve derleme belgeleri güncellendi.
- `make clean` artık kaynak AppImage simgesini ve desktop dosyasını silmez.

### Doğrulama
- `go test ./...`, `go vet ./...` ve Linux/macOS derleme kontrolleri.
- Windows'ta `amd` ve `arm` sunucularında gerçek etkileşimli SSH/ConPTY testi.
- Windows kurulum/kaldırma/yeniden kurulum, kullanıcı verilerinin korunması,
  EXE sürüm bilgisi, Başlat menüsü kısayolu ve SHA-256 kontrolleri geçti.
- Linux GUI çalışma zamanı testi bu Windows ortamında yapılmadı.
  Linux AppImage bu yayına henüz eklenmedi.
