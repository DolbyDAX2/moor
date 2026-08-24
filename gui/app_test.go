package gui

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"moor/store"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	st, err := store.Load(filepath.Join(t.TempDir(), "servers.json"))
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	return &App{st: st}
}

func TestSaveListDelete(t *testing.T) {
	app := newTestApp(t)

	err := app.SaveServer(ServerInput{
		Name: "test-srv", Host: "1.2.3.4", Port: 22,
		User: "root", AuthType: "key", KeyPath: "/tmp/id_ed25519",
	})
	if err != nil {
		t.Fatalf("SaveServer: %v", err)
	}

	got := app.Servers()
	if len(got) != 1 {
		t.Fatalf("beklenen 1 sunucu, gelen %d", len(got))
	}
	sv := got[0]
	if sv.Host != "1.2.3.4" || sv.Port != 22 || sv.User != "root" {
		t.Errorf("alanlar yanlış: %+v", sv)
	}
	if sv.AuthType != "key" {
		t.Errorf("authType anahtar olmalı, %q", sv.AuthType)
	}
	if sv.SSHCommand == "" || sv.DisplayHost == "" {
		t.Errorf("hesaplanmış alanlar boş: %+v", sv)
	}

	// Aynı id ile güncelle.
	in := app.GetServer(sv.ID)
	in.Name = "test-srv-2"
	if err := app.SaveServer(in); err != nil {
		t.Fatalf("SaveServer (update): %v", err)
	}
	if n := len(app.Servers()); n != 1 {
		t.Fatalf("güncelleme tekil kalmalı, %d sunucu var", n)
	}
	if app.Servers()[0].Name != "test-srv-2" {
		t.Errorf("ad güncellenmedi: %q", app.Servers()[0].Name)
	}

	if err := app.DeleteServer(sv.ID); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}
	if n := len(app.Servers()); n != 0 {
		t.Fatalf("silme sonrası liste boş olmalı, %d var", n)
	}
}

func TestSaveServerValidation(t *testing.T) {
	app := newTestApp(t)
	if err := app.SaveServer(ServerInput{Name: "x", Host: "", Port: 22}); err == nil {
		t.Error("host boşken hata bekleniyordu")
	}
}

func TestExportImportJSON(t *testing.T) {
	app := newTestApp(t)
	if err := app.SaveServer(ServerInput{Name: "yedek", Host: "5.6.7.8", Port: 22, AuthType: "password", Password: "pw"}); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}

	out := app.ExportJSON()
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("ExportJSON geçersiz JSON: %v", err)
	}
	if len(parsed) != 1 {
		t.Fatalf("beklenen 1 kayıt, %d", len(parsed))
	}

	// İkinci bir App'e aynı metni içe aktar.
	app2 := newTestApp(t)
	res, err := app2.ImportJSON(out)
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if res.Added != 1 || res.Skipped != 0 {
		t.Errorf("beklenen added=1 skipped=0, gelen %+v", res)
	}
	// Tekrar içe aktarmak atlamalı.
	res2, err := app2.ImportJSON(out)
	if err != nil {
		t.Fatalf("ImportJSON (2): %v", err)
	}
	if res2.Added != 0 || res2.Skipped != 1 {
		t.Errorf("beklenen added=0 skipped=1, gelen %+v", res2)
	}
}
