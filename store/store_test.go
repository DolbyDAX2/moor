package store

import (
	"os"
	"path/filepath"
	"testing"

	"moor/model"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(st.List()) != 0 {
		t.Fatalf("boş depo bekleniyordu")
	}

	sv1 := &model.Server{Name: "oracle-prod", Host: "203.0.113.1", Port: 22, User: "root", KeyPath: "~/.ssh/oracle_key"}
	sv1.Normalize()
	if err := sv1.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	sv2 := &model.Server{Name: "staging", Host: "example.com", Port: 2222, User: "ubuntu"}
	sv2.Normalize()
	st.Add(sv1)
	st.Add(sv2)
	if err := st.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	st2, err := Load(path)
	if err != nil {
		t.Fatalf("Load 2: %v", err)
	}
	if len(st2.List()) != 2 {
		t.Fatalf("2 sunucu bekleniyordu, %d geldi", len(st2.List()))
	}
	// ad sıralaması: oracle-prod < staging
	if st2.List()[0].Name != "oracle-prod" {
		t.Fatalf("sıralama hatalı: %s", st2.List()[0].Name)
	}

	sv1b, i := st2.FindByName("ORACLE-PROD")
	if i < 0 || sv1b == nil {
		t.Fatal("büyük/küçük harf duyarsız arama başarısız")
	}
	st2.Remove(sv1b.ID)
	if len(st2.List()) != 1 {
		t.Fatal("Remove başarısız")
	}
	if err := st2.Save(); err != nil {
		t.Fatalf("Save 2: %v", err)
	}
}

func TestStoreFilePermission(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	st, _ := Load(path)
	if err := st.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("dosya izni 0600 olmalı, %o geldi", fi.Mode().Perm())
	}
}
