package syncx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"moor/model"
)

// fakeAPI, gerçek API'nin davranışını (state/push/409) taklit eder.
type fakeAPI struct {
	version     int64
	data        []*model.Server
	passChanged bool
	accDeleted  bool
}

func (f *fakeAPI) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/login" && r.Method == "POST":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok-123", "username": "u"})
		case r.URL.Path == "/api/v1/sync/state" && r.Method == "GET":
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"version": f.version, "servers": f.data,
			})
		case r.URL.Path == "/api/v1/sync/push" && r.Method == "PUT":
			var in struct {
				BaseVersion int64           `json:"baseVersion"`
				Servers     []*model.Server `json:"servers"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				w.WriteHeader(400)
				return
			}
			if in.BaseVersion != f.version {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(409)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error": "çakışma", "version": f.version, "servers": f.data,
				})
				return
			}
			f.version++
			f.data = in.Servers
			_ = json.NewEncoder(w).Encode(map[string]int64{"version": f.version})
		case r.URL.Path == "/api/v1/password" && r.Method == "POST":
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(401)
				return
			}
			var in struct {
				OldPassword string `json:"oldPassword"`
				NewPassword string `json:"newPassword"`
			}
			_ = json.NewDecoder(r.Body).Decode(&in)
			if in.OldPassword == "yanlis" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(401)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "mevcut şifre hatalı"})
				return
			}
			f.passChanged = true
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		case r.URL.Path == "/api/v1/me" && r.Method == "DELETE":
			if r.Header.Get("Authorization") != "Bearer tok-123" {
				w.WriteHeader(401)
				return
			}
			f.accDeleted = true
			_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
		default:
			w.WriteHeader(404)
		}
	}
}

func TestChangePasswordAndDeleteAccount(t *testing.T) {
	fake := &fakeAPI{version: 1, data: []*model.Server{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/", fake.handler(t))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	c, err := NewClient(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login("u", "p"); err != nil {
		t.Fatal(err)
	}
	// yanlış mevcut şifre reddedilir
	if err := c.ChangePassword("yanlis", "yeni-sifre-123"); err == nil {
		t.Fatal("yanlış eski şifreyle değişim başarılı oldu (asla olmamalı)")
	}
	// doğru ise kabul edilir
	if err := c.ChangePassword("dogru", "yeni-sifre-123"); err != nil {
		t.Fatalf("doğru eski şifreyle değişim başarısız: %v", err)
	}
	if !fake.passChanged {
		t.Fatal("sunucu tarafında şifre değişimi işareti yok")
	}
	// hesap silme
	if err := c.DeleteAccount(); err != nil {
		t.Fatalf("hesap silinemedi: %v", err)
	}
	if !fake.accDeleted {
		t.Fatal("sunucu tarafında hesap silme işareti yok")
	}
}

func TestClientFlowAndConflict(t *testing.T) {
	fake := &fakeAPI{version: 1, data: []*model.Server{ms("r", "remote", time.Now())}}
	mux := http.NewServeMux()
	mux.HandleFunc("/", fake.handler(t))
	ts2 := httptest.NewServer(mux)
	defer ts2.Close()

	c, err := NewClient(ts2.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Login("u", "p"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if c.Token() != "tok-123" {
		t.Fatal("token ayarlanmadı")
	}

	// 1) doğru baseVersion ile push
	local := []*model.Server{ms("l", "local", time.Now())}
	v, err := c.Push(1, local)
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	if v != 2 {
		t.Fatalf("yeni sürüm 2 bekleniyor, %d geldi", v)
	}

	// 2) eski baseVersion ile push -> ConflictError + sunucunun listesi
	_, err = c.Push(1, local)
	if !IsConflict(err) {
		t.Fatalf("çakışma bekleniyor, gelen hata: %v", err)
	}
	ce := err.(*ConflictError)
	if ce.Version != 2 || len(ce.Servers) != 1 || ce.Servers[0].ID != "l" {
		t.Fatalf("çakışma detayı yanlış: %+v", ce)
	}

	// 3) SyncStore: state sürümü == lastVersion -> doğrudan push
	fake2 := &fakeAPI{version: 5, data: []*model.Server{ms("x", "x", time.Now())}}
	ts3 := httptest.NewServer(http.HandlerFunc(fake2.handler(t)))
	defer ts3.Close()
	c2, _ := NewClient(ts3.URL)
	c2.SetToken("tok-123")
	merged, sum, err := SyncStore(c2, local, 5)
	if err != nil {
		t.Fatalf("SyncStore: %v", err)
	}
	if len(merged) != 1 || merged[0].ID != "l" {
		t.Fatalf("merged yanlış: %+v", merged)
	}
	if sum.Version != 6 {
		t.Fatalf("sürüm 6 bekleniyor, %d geldi", sum.Version)
	}

	// 4) SyncStore: state sürümü farklı -> merge
	fake3 := &fakeAPI{version: 7, data: []*model.Server{ms("y", "y", time.Now())}}
	ts4 := httptest.NewServer(http.HandlerFunc(fake3.handler(t)))
	defer ts4.Close()
	c3, _ := NewClient(ts4.URL)
	c3.SetToken("tok-123")
	local2 := []*model.Server{ms("l", "local", time.Now()), ms("z", "z", time.Now())}
	merged3, sum3, err := SyncStore(c3, local2, 1)
	if err != nil {
		t.Fatalf("SyncStore(merge): %v", err)
	}
	ids := map[string]bool{}
	for _, s := range merged3 {
		ids[s.ID] = true
	}
	if !ids["l"] || !ids["z"] || !ids["y"] {
		t.Fatalf("merge eksik: %v", ids)
	}
	if sum3.Version != 8 {
		t.Fatalf("sürüm 8 bekleniyor, %d geldi", sum3.Version)
	}
	if sum3.RemoteOnly != 1 || sum3.LocalOnly != 2 {
		t.Fatalf("özet sayıları yanlış: %+v", sum3)
	}
}
