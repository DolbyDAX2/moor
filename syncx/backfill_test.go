package syncx

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"moor/model"
)

func TestBackfillKeyContent(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "id_rsa")
	if err := os.WriteFile(keyFile, []byte("PRIVATE-KEY-DATA"), 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	servers := []*model.Server{
		{ID: "a", Name: "a", Host: "1.2.3.4", Port: 22, AuthType: model.AuthKey, KeyPath: keyFile},
		{ID: "b", Name: "b", Host: "5.6.7.8", Port: 22, AuthType: model.AuthPassword, Password: "x"},
		{ID: "c", Name: "c", Host: "9.10.11.12", Port: 22, AuthType: model.AuthKey, KeyPath: filepath.Join(dir, "yok.pem")},
	}
	for _, s := range servers {
		s.CreatedAt = now
		s.UpdatedAt = now
	}

	before := servers[0].UpdatedAt
	BackfillKeyContent(servers)

	if servers[0].KeyContent != "PRIVATE-KEY-DATA" {
		t.Fatalf("anahtar içeriği doldurulmadı: %q", servers[0].KeyContent)
	}
	if !servers[0].UpdatedAt.After(before) {
		t.Fatal("içerik değişince updated_at güncellenmeli (eski kopyalar merge'de kaybetmeli)")
	}
	if servers[1].KeyContent != "" {
		t.Fatalf("parola sunucusuna içerik yazıldı: %q", servers[1].KeyContent)
	}
	if servers[2].KeyContent != "" {
		t.Fatalf("okunamayan dosyada içerik beklenmez: %q", servers[2].KeyContent)
	}

	// İkinci çağrı: içerik aynıysa güncelleme zamanı değişmemeli.
	stable := servers[0].UpdatedAt
	BackfillKeyContent(servers)
	if !servers[0].UpdatedAt.Equal(stable) {
		t.Fatal("içerik değişmede updated_at oynamamalı")
	}

	// Dosya döndürülürse içerik + zaman güncellenmeli.
	if err := os.WriteFile(keyFile, []byte("ROTATED-KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	BackfillKeyContent(servers)
	if servers[0].KeyContent != "ROTATED-KEY" {
		t.Fatalf("döndürülen anahtar okunmadı: %q", servers[0].KeyContent)
	}
	if !servers[0].UpdatedAt.After(stable) {
		t.Fatal("içerik değişince updated_at tekrar güncellenmeli")
	}
}

func TestMergeInheritsKeyContent(t *testing.T) {
	t0 := time.Now().Add(-time.Hour)
	t1 := t0.Add(time.Minute)

	// Yerel: eski, anahtar içeriği YOK. Uzak: aynı/dağı zaman, içerik VAR.
	local := []*model.Server{{
		ID: "a", Name: "a", Host: "1.2.3.4", Port: 22,
		AuthType: model.AuthKey, KeyPath: "/x", CreatedAt: t0, UpdatedAt: t0,
	}}
	remote := []*model.Server{{
		ID: "a", Name: "a", Host: "1.2.3.4", Port: 22,
		AuthType: model.AuthKey, KeyPath: "/x", KeyContent: "KEYDATA",
		CreatedAt: t0, UpdatedAt: t0, // berabere -> yerel kazanır
	}}

	merged := MergeServers(local, remote)
	if len(merged) != 1 {
		t.Fatalf("beklenen 1 kayıt, %d geldi", len(merged))
	}
	if merged[0].KeyContent != "KEYDATA" {
		t.Fatalf("içerik devralınmadı: %q", merged[0].KeyContent)
	}

	// Zamanlama tersi de: uzak daha yeni + içerikli -> zaten kazanır.
	remote[0].UpdatedAt = t1
	merged = MergeServers(local, remote)
	if merged[0].KeyContent != "KEYDATA" {
		t.Fatalf("yeni uzak kayıt içeriği taşımalı: %q", merged[0].KeyContent)
	}

	// Kazananın (berabere -> yerel) zaten içeriği varsa kaybedenininki ezmez.
	remote[0].UpdatedAt = t0 // berabere
	local[0].KeyContent = "LOCAL-KEY"
	merged = MergeServers(local, remote)
	if merged[0].KeyContent != "LOCAL-KEY" {
		t.Fatalf("mevcut içerik ezilmemeli: %q", merged[0].KeyContent)
	}

	// Parola sunucusu etkilenmez.
	pLocal := []*model.Server{{ID: "p", Name: "p", Host: "h", Port: 22, AuthType: model.AuthPassword, UpdatedAt: t0}}
	pRemote := []*model.Server{{ID: "p", Name: "p", Host: "h", Port: 22, AuthType: model.AuthPassword, KeyContent: "X", UpdatedAt: t0}}
	merged = MergeServers(pLocal, pRemote)
	if merged[0].KeyContent != "" {
		t.Fatalf("parola sunucusuna içerik devralınmaz: %q", merged[0].KeyContent)
	}
}
