// Package store sunucu listesinin JSON dosyasında kalıcı tutulmasını sağlar.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"moor/model"
)

// Store sunucu listesini bir JSON dosyasında tutar.
type Store struct {
	Path string

	servers []*model.Server
}

// configDir platforma uygun konfigürasyon dizinini döndürür:
// Linux/macOS: $XDG_CONFIG_HOME (varsayılan ~/.config), Windows: %APPDATA%.
func configDir() (string, error) {
	return os.UserConfigDir()
}

// DefaultPath varsayılan veri dosyası yolunu döndürür:
// Linux: ~/.config/moor/servers.json, Windows: %APPDATA%\moor\servers.json
func DefaultPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "moor", "servers.json"), nil
}

// MigrateLegacyPath, eski sunucukontrol dizininden yeni moor dizinine
// veri taşır (yalnızca yeni dosya yoksa).
func MigrateLegacyPath() {
	dir, err := configDir()
	if err != nil {
		return
	}
	oldFile := filepath.Join(dir, "sunucukontrol", "servers.json")
	newFile := filepath.Join(dir, "moor", "servers.json")
	if _, err := os.Stat(newFile); err == nil {
		return // yeni dosya zaten var
	}
	data, err := os.ReadFile(oldFile)
	if err != nil {
		return // eski dosya yok — yapılacak bir şey yok
	}
	_ = os.MkdirAll(filepath.Dir(newFile), 0o700)
	_ = os.WriteFile(newFile, data, 0o600)
	_ = os.RemoveAll(filepath.Join(dir, "sunucukontrol"))
}

// Load deposunu okur; dosya yoksa boş depo döndürür.
func Load(path string) (*Store, error) {
	// Varsayılan yol kullanılıyorsa, eski sunucukontrol dizinindeki veriyi taşı.
	if dp, err := DefaultPath(); err == nil && path == dp {
		MigrateLegacyPath()
	}
	s := &Store{Path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sunucu listesi okunamadı: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return s, nil
	}
	var servers []*model.Server
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("sunucu listesi bozuk: %w", err)
	}
	for _, sv := range servers {
		sv.Normalize()
	}
	s.servers = servers
	return s, nil
}

// Save depoyu atomik biçimde yazar (dosya 0600 izinli).
func (s *Store) Save() error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config dizini oluşturulamadı: %w", err)
	}
	data, err := json.MarshalIndent(s.servers, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".servers-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename başarılıysa iş yapmaz
	_ = tmp.Chmod(0o600)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return err
	}
	return os.Chmod(s.Path, 0o600)
}

// List sunucuları adıya göre sıralı döndürür.
func (s *Store) List() []*model.Server {
	out := make([]*model.Server, len(s.servers))
	copy(out, s.servers)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// Add sunucuyu ekler.
func (s *Store) Add(sv *model.Server) { s.servers = append(s.servers, sv) }

// Remove kimliğe göre siler.
func (s *Store) Remove(id string) {
	for i, sv := range s.servers {
		if sv.ID == id {
			s.servers = append(s.servers[:i], s.servers[i+1:]...)
			return
		}
	}
}

// FindByID kimliğe göre arar.
func (s *Store) FindByID(id string) *model.Server {
	for _, sv := range s.servers {
		if sv.ID == id {
			return sv
		}
	}
	return nil
}

// FindByName ada göre (büyük/küçük harf duyarsız) arar.
func (s *Store) FindByName(name string) (*model.Server, int) {
	for i, sv := range s.servers {
		if strings.EqualFold(sv.Name, name) {
			return sv, i
		}
	}
	return nil, -1
}

// ReplaceAll listeyi tamamen verilen içerikle değiştirir (senkron için).
func (s *Store) ReplaceAll(servers []*model.Server) {
	s.servers = servers
}

// Update aynı kimliğe sahip sunucuyu değiştirir; yoksa ekler.
func (s *Store) Update(sv *model.Server) {
	for i, old := range s.servers {
		if old.ID == sv.ID {
			s.servers[i] = sv
			return
		}
	}
	s.servers = append(s.servers, sv)
}
