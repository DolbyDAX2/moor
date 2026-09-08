package syncx

import (
	"os"
	"time"

	"moor/model"
	"moor/sshx"
)

// BackfillKeyContent, anahtarla doğrulanan sunucular için key_content
// alanını KeyPath'teki dosyanın güncel içeriğiyle doldurur.
//
// Neden: Android tarafında masaüstü dosya yolu anlamsızdır; anahtar
// içeriği senkron verisiyle taşındığında telefon anahtarı doğrudan
// kullanabilir ve kullanıcıya her seferinde sorulmaz.
//
// Okunamayan dosyalar sessizce atlanır (mevcut davranış bozulmaz);
// dosya değiştiyse içerik güncel tutulur (her sync'te taze okunur).
// İçerik DEĞİŞİRSE UpdatedAt güncellenir: böylece diğer cihazdaki
// eski (içeriksiz) kopya merge'de kaybeder ve yeni içerik yayılır.
func BackfillKeyContent(servers []*model.Server) {
	for _, s := range servers {
		if s.AuthType != model.AuthKey {
			continue
		}
		path := sshx.ExpandPath(s.KeyPath)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		if s.KeyContent == content {
			continue
		}
		s.KeyContent = content
		s.UpdatedAt = time.Now()
	}
}
