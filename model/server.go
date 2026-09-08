// Package model uygulama verilerini tanımlar.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/user"
	"strings"
	"time"
)

// AuthType kimlik doğrulama yöntemini belirtir.
type AuthType string

const (
	// AuthKey: özel anahtar yolu -i ile verilir.
	AuthKey AuthType = "key"
	// AuthPassword: kayıtlı şifre (sshpass) veya etkileşimli prompt,
	// ayrıca ssh'nin varsayılan anahtarları/agent da denenir.
	AuthPassword AuthType = "password"
)

// Server uygulamaya kaydedilmiş uzak bir sunucudur.
type Server struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	User     string   `json:"user,omitempty"`
	AuthType AuthType `json:"auth_type"`
	KeyPath  string   `json:"key_path,omitempty"`
	// KeyContent, anahtar dosyasının içeriğini taşır; böylece dosya
	// sisteminin olmadığı cihazlarda (Android) anahtar senkronla
	// kullanılabilir. Senkron öncesi [syncx.BackfillKeyContent]
	// ile KeyPath'ten doldurulur.
	KeyContent    string    `json:"key_content,omitempty"`
	KeyPassphrase string    `json:"key_passphrase,omitempty"`
	Password      string    `json:"password,omitempty"`
	ExtraSSHArgs  string    `json:"extra_ssh_args,omitempty"`
	Notes         string    `json:"notes,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NewID kısa rastgele bir kimlik üretir.
func NewID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// Normalize varsayılan değerleri doldurur (port, kimlik türü, id).
func (s *Server) Normalize() {
	if s.Port == 0 {
		s.Port = 22
	}
	if s.AuthType == "" {
		if strings.TrimSpace(s.KeyPath) != "" {
			s.AuthType = AuthKey
		} else {
			s.AuthType = AuthPassword
		}
	}
	if s.ID == "" {
		s.ID = NewID()
	}
}

// Validate zorunlu alanları kontrol eder.
func (s *Server) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("ad boş olamaz")
	}
	if strings.TrimSpace(s.Host) == "" {
		return fmt.Errorf("host (IP veya alan adı) boş olamaz")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port 1-65535 arasında olmalı")
	}
	if s.AuthType == AuthKey && strings.TrimSpace(s.KeyPath) == "" {
		return fmt.Errorf("anahtar kimliği için anahtar yolu gerekli")
	}
	return nil
}

// CurrentUser yerel işletim sistemi kullanıcı adını döndürür.
func CurrentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "root"
}

// EffectiveUser sunucunun kullanıcı adı; boşsa yerel kullanıcıyı döndürür.
func (s *Server) EffectiveUser() string {
	if strings.TrimSpace(s.User) != "" {
		return s.User
	}
	return CurrentUser()
}

// Target "user@host" biçiminde ssh hedefini döndürür.
func (s *Server) Target() string { return s.EffectiveUser() + "@" + s.Host }

// DisplayHost portu 22 değilse "host:port" olarak döndürür.
func (s *Server) DisplayHost() string {
	if s.Port == 22 {
		return s.Host
	}
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// AuthLabel kimlik türünün kısa etiketini döndürür.
func (s *Server) AuthLabel() string {
	if s.AuthType == AuthKey {
		return "anahtar"
	}
	return "şifre"
}
