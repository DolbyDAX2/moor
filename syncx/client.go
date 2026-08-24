// Package syncx, Moor istemcisinin senkronizasyon API'siyle
// (moor-api) konuşan katmanıdır: kimlik doğrulama, durum alma,
// push ve akıllı birleştirme (merge).
package syncx

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"moor/model"
)

// DefaultBaseURL, kullanıcı adres belirtmezse kullanılan varsayılan sunucu.
const DefaultBaseURL = "https://moorapi.fatihdurdu.xyz"

// Client senkronizasyon API istemcisi.
type Client struct {
	base string
	tok  string
	http *http.Client
}

// State sunucudaki anlık durum (GET /sync/state).
type State struct {
	Version int64           `json:"version"`
	At      string          `json:"updatedAt"`
	Servers []*model.Server `json:"servers"`
}

// ConflictError, push sırasında sürüm çakışması olduğunu bildirir;
// sunucudaki güncel durumu taşır (merge için).
type ConflictError struct {
	Want    int64 // istenen baseVersion
	Version int64 // sunucudaki gerçek sürüm
	Servers []*model.Server
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("çakışma: istenen sürüm %d, sunucudaki sürüm %d", e.Want, e.Version)
}

// IsConflict hatanın sürüm çakışması olup olmadığını sorar.
func IsConflict(err error) bool {
	_, ok := err.(*ConflictError)
	return ok
}

// NewClient API adresiyle istemci oluşturur.
func NewClient(baseURL string) (*Client, error) {
	u := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("geçersiz API adresi: %s", baseURL)
	}
	// Zaman aşımı 60s: düşük kaynaklı VPS'te yük anında yanıtlar yavaşlayabilir.
	return &Client{base: u, http: &http.Client{Timeout: 60 * time.Second}}, nil
}

// SetToken oturum token'ını ayarlar.
func (c *Client) SetToken(t string) { c.tok = t }

// Token mevcut oturum token'ını döndürür.
func (c *Client) Token() string { return c.tok }

// request ham yanıt durumu + gövdeyi döndüren çekirdek.
func (c *Client) request(method, path string, in any) (int, []byte, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, nil, err
		}
		body = strings.NewReader(string(b))
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.tok != "" {
		req.Header.Set("Authorization", "Bearer "+c.tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("API'ye ulaşılamadı: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return 0, nil, fmt.Errorf("yanıt okunamadı: %w", err)
	}
	return resp.StatusCode, data, nil
}

func (c *Client) do(method, path string, in any, out any) error {
	code, data, err := c.request(method, path, in)
	if err != nil {
		return err
	}
	switch {
	case code >= 200 && code < 300:
		if out != nil && len(data) > 0 {
			return json.Unmarshal(data, out)
		}
		return nil
	default:
		msg := http.StatusText(code)
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(data, &e); err == nil && e.Error != "" {
			msg = e.Error
		}
		return fmt.Errorf("%s", msg)
	}
}

// Login kullanıcı adı + şifreyle oturum açar.
func (c *Client) Login(username, password string) error {
	var out struct {
		Token    string `json:"token"`
		Username string `json:"username"`
	}
	if err := c.do("POST", "/api/v1/login", map[string]string{
		"username": username, "password": password,
	}, &out); err != nil {
		return err
	}
	c.tok = out.Token
	return nil
}

// Register yeni hesap oluşturur ve oturum açar.
func (c *Client) Register(username, password string) error {
	var out struct {
		Token    string `json:"token"`
		Username string `json:"username"`
	}
	if err := c.do("POST", "/api/v1/register", map[string]string{
		"username": username, "password": password,
	}, &out); err != nil {
		return err
	}
	c.tok = out.Token
	return nil
}

// Logout oturumu kapatır (best-effort).
func (c *Client) Logout() error {
	return c.do("POST", "/api/v1/logout", nil, nil)
}

// ChangePassword şifreyi değiştirir (mevcut oturum korunur, diğer
// cihazlardaki oturumlar kapatılır).
func (c *Client) ChangePassword(oldPassword, newPassword string) error {
	return c.do("POST", "/api/v1/password", map[string]string{
		"oldPassword": oldPassword, "newPassword": newPassword,
	}, nil)
}

// DeleteAccount hesabın tamamını sunucudan siler (senkron verisiyle).
func (c *Client) DeleteAccount() error {
	return c.do("DELETE", "/api/v1/me", nil, nil)
}

// GetState sunucudaki güncel durumu döndürür.
func (c *Client) GetState() (*State, error) {
	var s State
	if err := c.do("GET", "/api/v1/sync/state", nil, &s); err != nil {
		return nil, err
	}
	if s.Servers == nil {
		s.Servers = []*model.Server{}
	}
	return &s, nil
}

// Push listeyi baseVersion üzerine yazar; yeni sürümü döndürür.
// Çakışmada *ConflictError döner (sunucudaki güncel listeyle).
func (c *Client) Push(baseVersion int64, servers []*model.Server) (int64, error) {
	code, data, err := c.request("PUT", "/api/v1/sync/push", map[string]any{
		"baseVersion": baseVersion,
		"servers":     servers,
	})
	if err != nil {
		return 0, err
	}
	switch {
	case code == http.StatusOK:
		var out struct {
			Version int64 `json:"version"`
		}
		if err := json.Unmarshal(data, &out); err != nil {
			return 0, err
		}
		return out.Version, nil
	case code == http.StatusConflict:
		var ce struct {
			Version int64           `json:"version"`
			Servers []*model.Server `json:"servers"`
		}
		if err := json.Unmarshal(data, &ce); err != nil {
			return 0, &ConflictError{Want: baseVersion, Version: -1}
		}
		return 0, &ConflictError{Want: baseVersion, Version: ce.Version, Servers: ce.Servers}
	default:
		msg := http.StatusText(code)
		var e struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(data, &e); err == nil && e.Error != "" {
			msg = e.Error
		}
		return 0, fmt.Errorf("%s", msg)
	}
}
