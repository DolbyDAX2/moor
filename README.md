# moor

Uzak sunucularınızı **kaydedip** terminalden ve masaüstü uygulamasından
**kontrol etmenizi** sağlayan tek binary araç. Go ile yazıldı; derleme dışında
hiçbir bağımlılığı yok (sistemde `ssh` yeterli).

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
çıkış yapar; klavye girdisi ve boyut değişimi gerçek bir PTY üzerinden akar.
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

- Kimlik bilgisi `~/.config/moor/credentials.json` (izin 0600) dosyasında tutulur.
- `moor sync`, yerel + uzak listeyi akıllıca birleştirir: aynı kayıttaki
  `updated_at` yenisi kazanır; yalnızca bir tarafta olanlar eklenir. Çakışma
  durumunda (409) sunucunun döndürdüğü güncel listeyle otomatik merge yapılır.
- GUI'de üst bardaki **☁ Senkron** butonu ile giriş/kayıt/çıkış ve "Senkronize Et"
unabilirsin.
- Otomatik senkron: GUI açılışta sessizce pull yapılır; listedeki değişiklikler
  (kaydet/sil/içe aktar) sonrası ~1,5 sn içinde otomatik push edilir. Giriş
  yapılmamışsa hiçbir şey olmaz.
- **☁ Senkron** panelinde ayrıca *Şifre değiştir* (mevcut + yeni şifre) ve
  *Hesabı sil* (buluttaki verisiyle birlikte) bulunur.
- Şifreler Argon2id ile hash'lenir; sunucu listesi (IP, parola vb.) `DATA_KEY`
  ile AES-256-GCM şifrelenerek saklanır. Dışa açık port yoktur; trafik yalnızca
  Cloudflare Tunnel üzerinden HTTPS olarak gelir.
- Şifreyi unutursan: e-posta sıfırlama yok (tasarım gereği). Yerel liste her
  zaman tek kaynak olduğu için çözüm basittir — hesabı silip yeniden kayıt
  olup `moor sync` yapman yeterli; verin buluta yeniden yüklenir.

## Kurulum / Derleme

Gereksinim: Go 1.24+ (sadece derleme sırasında). GUI için `webkit2gtk-4.1` +
`gtk3` (CachyOS/Arch'ta varsayılan paketler).

```bash
make build            # ./moor binarysi (CLI + GUI tek binary)
./moor               # masaüstü uygulamasını aç (argümansız = GUI)
./moor ls            # CLI: kayıtlı sunucular
make install          # ~/.local/bin/moor
```

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

Sunucular JSON olarak saklanır: `~/.config/moor/servers.json` (izin 0600).
Yol `MOOR_CONFIG` ortam değişkeniyle değiştirilebilir. Eski
`~/.config/sunucukontrol/` dizinindeki veri ilk açılışta otomatik taşınır.
