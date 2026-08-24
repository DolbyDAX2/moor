package syncx

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Credentials, senkronizasyon hesabının istemci tarafındaki kaydı
// (~/.config/moor/credentials.json, 0600).
type Credentials struct {
	ServerURL       string    `json:"server"`
	Username        string    `json:"username"`
	Token           string    `json:"token"`
	LastSyncVersion int64     `json:"lastSyncVersion"`
	LastSyncAt      time.Time `json:"lastSyncAt,omitempty"`
}

// LoggedIn hesap bilgisi olup olmadığını sorar.
func (c *Credentials) LoggedIn() bool { return c != nil && c.Token != "" }

// CredentialsPath kimlik dosyası yolunu döndürür (store ile aynı dizin).
func CredentialsPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "moor", "credentials.json"), nil
}

// LoadCredentials kimlik dosyasını okur; yoksa boş Credentials döndürür.
func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return &Credentials{}, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Credentials{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kimlik dosyası okunamadı: %w", err)
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("kimlik dosyası bozuk: %w", err)
	}
	return &c, nil
}

// SaveCredentials kimlik dosyasını 0600 izinle yazar.
func (c *Credentials) Save() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// ClearCredentials kimlik dosyasını siler.
func ClearCredentials() error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
