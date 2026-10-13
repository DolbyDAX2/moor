# moor

**Güncel sürüm: v0.1.2** · [İndir](https://github.com/DolbyDAX2/moor/releases/latest)

Uzak sunucularınızı **kaydedip** terminalden ve masaüstü uygulamasından
**kontrol etmenizi** sağlayan tek binary araç. Go ile yazıldı; derleme dışında
CLI için sistemde `ssh` gerekir; GUI Linux'ta GTK/WebKitGTK,
Windows'ta Microsoft Edge WebView2 Runtime kullanır.

## Özellikler

- Sunucu kaydetme: ad, host, port, kullanıcı, kimlik yöntemi (özel anahtar yolu
  veya şifre), etiketler, notlar, ek SSH argümanları
- Kayıtlı sunucuya hızlı SSH bağlanma ve uzaktan komut çalıştırma
- `scp`, `sftp`, `rsync` ve yerel port yönlendirme (`fwd`)
- TCP ping / gecikme ölçümü ve gerçek SSH kimlik doğrulama testi
- Mevcut `~/.ssh/config` dosyasından sunucu içe aktarma
- JSON yedek alma / geri yükleme (içe / dışa aktarma)

## Grafik Arayüz (GUI)

Wails v2 tabanlı masaüstü uygulaması — tarayıcı **değil**, istemci bir webview
içeren gerçek bir masaüstü penceresidir. CLI ile **aynı veri dosyasını**
kullanır, aynı anda kullanılabilir.

GUI'de neler var:

- Sunucu listesi + canlı ping durumu (açılışta otomatik)
- Arama/filtre, ekleme/düzenleme/silme (onaylı)
- `~/.ssh/config` içe aktarma, JSON yedek alma / geri yükleme
- Seçili sunucu için: SSH komutunu panoya kopyalama, gerçek SSH oturum testi,
  **gömülü SSH terminali** (pencere içi, xterm.js + PTY), sistem terminalinde
  (Konsole vb.) SSH açma

Gömülü terminal: `⚡ SSH Bağlan` ile seçili sunucuya doğrudan pencerenin içinde
bağlantı açar; klavye girdisi ve boyut değişimi Linux'ta PTY,
Windows'ta ConPTY üzerinden akar. Birden fazla terminal sekmesi açılabilir.
Şifreli sunucularda şifre istemi bu ekranda sorulur (şifre log'a/dosyaya yazılmaz).

## Kullanım

```bash
moor                 # masaüstü (GUI) uygulaması
moor gui             # aynısı
moor ls              # kayıtlı sunucular
moor ssh AD          # SSH oturumu
moor run AD KOMUT... # uzaktan komut
moor ping all        # tümüne TCP ping
moor test AD         # SSH kimlik doğrulama testi
moor help            # tüm komutlar
```

## Bulut Senkron (seçmeli)

Sunucu listesini birden çok cihaz arasında senkronize etmek için kendi
`moor-api` sunucunu kullan. (Ayrı bir modül: bu repo'nun yanında `../api`.)

```bash
# Hesabın yoksa kayıt ol (API adresi sorulmaz; uygulama kendi
# sunucusunu bilir — moorapi.fatihdurdu.xyz):
moor register --user AD --pass ŞİFRE

# Listeyi bulutla eşitle (çift yönlü, akıllı merge):
moor sync

# Oturumu kapat:
moor logout
```

- Kimlik bilgisi Linux'ta `~/.config/moor/credentials.json` (izin 0600),
  Windows'ta `%APPDATA%\moor\credentials.json` dosyasında tutulur.
- `moor sync`, yerel + uzak listeyi akıllıca birleştirir: aynı kayıttaki
  `updated_at` yenisi kazanır; yalnızca bir tarafta olanlar eklenir. Çakışma
  durumunda (409) sunucunun döndürdüğü güncel listeyle otomatik merge yapılır.
- Anahtarla doğrulanan sunucularda `key_content` alanı, her sync öncesi
  `key_path` dosyasının güncel içeriğiyle doldurulur (syncx/backfill.go).
  Böylece Android uygulaması dosya yolu olmadan anahtarı doğrudan kullanır;
  dosya değişirse içerik de değişir. Bu alan API tarafından saydam taşınır
  (aynı şifreli blob içinde), API değişikliği gerektirmez.
- GUI'de üst bardaki **☁ Senkron** butonu ile giriş/kayıt/çıkış ve "Senkronize Et"
işlemlerini kullanabilirsin.
- Otomatik senkron: GUI açılışta sessizce pull yapılır; listedeki değişiklikler
  (kaydet/sil/içe aktar) sonrası ~1,5 sn içinde otomatik push edilir. Giriş
  yapılmamışsa hiçbir şey olmaz. GUI ve CLI'de giriş/kayıt sonrası da
  otomatik eşitleme yapılır.
- Windows'ta başka cihazdan gelen Linux anahtar yolu geçersiz olsa bile
  senkronlanmış `key_content` geçici anahtar dosyasına aktarılarak kullanılır.
  Özel anahtarlar hassas veridir; yalnızca güvendiğin cihazlarda oturum aç.
- **☁ Senkron** panelinde ayrıca *Şifre değiştir* (mevcut + yeni şifre) ve
  *Hesabı sil* (buluttaki verisiyle birlikte) bulunur.
- Şifreler Argon2id ile hash'lenir; sunucu listesi (IP, parola vb.) `DATA_KEY`
  ile AES-256-GCM şifrelenerek saklanır. Dışa açık port yoktur; trafik yalnızca
  Cloudflare Tunnel üzerinden HTTPS olarak gelir.
- Şifreyi unutursan: e-posta sıfırlama yok (tasarım gereği). Yerel liste her
  zaman tek kaynak olduğu için çözüm basittir — hesabı silip yeniden kayıt
  olup `moor sync` yapman yeterli; verin buluta yeniden yüklenir.

## Windows kurulumu (x64)

1. [v0.1.2 Release](https://github.com/DolbyDAX2/moor/releases/tag/v0.1.2)
   sayfasından `moor-0.1.2-windows-amd64-setup.exe` indir ve çalıştır.
2. Uygulama kullanıcı hesabına `%LOCALAPPDATA%\Programs\Moor` altında kurulur;
   yönetici izni istemez. Başlat menüsü kısayolu, isteğe bağlı masaüstü kısayolu
   ve kaldırıcı eklenir. Güncellemek için yeni kurulum dosyasını çalıştır.
3. Kurulum Microsoft'un imzalı WebView2 yükleyicisini çalıştırır.
   Runtime yoksa indirilmesi için Internet bağlantısı gerekir.
4. SSH için **OpenSSH Client** etkin olmalıdır: Windows Ayarları → İsteğe
   bağlı özellikler → OpenSSH Client. Gerekirse yönetici PowerShell'de:

```powershell
Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0
```

Desteklenen sistem: Windows 10 1809+ / Windows 11, x64. Windows'taki gömülü
terminal sistemin OpenSSH istemcisini tercih eder; yoksa PATH'teki `ssh` aranır.
`rsync` ayrı kurulum veya WSL gerektirir; Windows sürümüyle birlikte gelmesi
varsayılmaz.

Kurulumsuz kullanım için `moor-0.1.2-windows-amd64.exe` indir. Aynı OpenSSH ve
WebView2 gereksinimleri geçerlidir. Kurulum PATH'i değiştirmez; CLI kullanımı:

```powershell
& "$env:LOCALAPPDATA\Programs\Moor\moor.exe" version
& "$env:LOCALAPPDATA\Programs\Moor\moor.exe" ls
```

Dosyaların SHA-256 değerleri Release'teki `SHA256SUMS.txt` içindedir:

```powershell
Get-FileHash .\moor-0.1.2-windows-amd64-setup.exe -Algorithm SHA256
```

Moor EXE ve kurulum dosyası kod imzalı değildir; Windows SmartScreen uyarısı
verebilir. Kaynak ve hash'i doğrulamadan bilinmeyen dosyaları çalıştırma.
Kaldırma, kayıtlı sunucuları ve kimlik bilgilerini silmez; bunlar `%APPDATA%\moor`
altında kalır.

## Kaynaktan derleme

Gereksinim: Go **1.26.5+** (yalnızca derleme sırasında).

### Linux

GUI için `webkit2gtk-4.1` + `gtk3` ve `pkg-config` gerekir. Ubuntu/Debian'da:

```bash
sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
```

CLI, SSH/scp/sftp için OpenSSH araçlarını kullanır.

```bash
make build            # ./moor binarysi (CLI + GUI tek binary)
./moor               # masaüstü uygulamasını aç (argümansız = GUI)
./moor ls            # CLI: kayıtlı sunucular
make install          # ~/.local/bin/moor
```

### Windows derleme ve kurulum paketi üretme

```powershell
go build -tags production -o moor.exe .
.\moor.exe version

# Kurulum paketi için ek derleme aracı:
winget install --id JRSoftware.InnoSetup --exact
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-windows-release.ps1
```

Script sürümü `main.go` içinden okur, EXE sürüm bilgisini ekler ve
`dist/windows/` altında taşınabilir EXE, kurulum EXE'si ve `SHA256SUMS.txt`
üretir. Paketleme sırasında Microsoft WebView2 bootstrapper indirilir ve
imzası doğrulanır. Kaynak kod ile kurulum tarifi repoda bulunur; derleme
çıktıları Git'e eklenmez. `make build-win` / `make release-win` de kullanılabilir.

### Testler

```bash
go test ./...
go vet ./...
```

Windows'ta gerçek SSH/ConPTY entegrasyon testi yalnızca açıkça istendiğinde
çalışır; belirtilen sunucu yerel kayıtlarında bulunmalıdır:

```powershell
$env:MOOR_TEST_SSH_SERVER = "arm"
go test ./gui -run TestWindowsSSHPTY -v -count=1
Remove-Item Env:MOOR_TEST_SSH_SERVER
```

Windows'ta `amd` ve `arm` üzerinde gerçek gömülü SSH giriş/çıkışı ve
boyutlandırma doğrulandı. Linux için derleme kontrolü yapıldı; bu sürümün
Linux GUI çalışma zamanı testi Windows ortamında yapılmadı.

## AppImage (kurulumsuz, taşınabilir)

Bilgisayara hiçbir şey kurmadan çalıştırmak için tek dosyalık AppImage üret:

```bash
# Bir kez: linuxdeploy (sistem paketi olarak kurulmaz, /tmp'ye indirilir)
curl -L -o /tmp/linuxdeploy.AppImage \
  https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage
chmod +x /tmp/linuxdeploy.AppImage

make appimage         # dist/Moor-x86_64.AppImage üretir (bağımlılıklar dahildir)
dist/Moor-x86_64.AppImage   # çift tıkla / terminalden çalıştır — kurulum yok
```

AppImage; GUI, webkit2gtk ve tüm paylaşımlı kütüphaneleri içinde taşır.
Yalnızca Linux x86_64 için derlenir (ilk çalıştırmada AppImage
"untrusted" uyarısı verirse sağ tık → "Extract" yerine imza doğrulaması;
FUSE yoksa `--appimage-extract-and-run` kullanılabilir).

## Veri

Sunucular JSON olarak saklanır:
- Linux: `~/.config/moor/servers.json` (izin 0600; `XDG_CONFIG_HOME` desteklenir).
- Windows: `%APPDATA%\moor\servers.json` (erişim Windows kullanıcı izinlerine bağlıdır).

Bu dosyalar özel anahtar/parola içerebilir; herkese açık depolara yükleme.
Yol `MOOR_CONFIG` ortam değişkeniyle değiştirilebilir. Eski
`~/.config/sunucukontrol/` dizinindeki veri ilk açılışta otomatik taşınır.
