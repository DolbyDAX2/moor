package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"moor/model"
	"moor/sshcfg"
	"moor/sshx"
	"moor/store"
	"moor/syncx"
)

// ServerView frontend'e döndürülen, hesaplanmış alanları da içeren sunucu görünümü.
type ServerView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	User         string   `json:"user"`
	AuthType     string   `json:"authType"`
	KeyPath      string   `json:"keyPath"`
	HasPassword  bool     `json:"hasPassword"`
	ExtraSSHArgs string   `json:"extraSSHArgs"`
	Notes        string   `json:"notes"`
	Tags         []string `json:"tags"`
	SSHCommand   string   `json:"sshCommand"`
	DisplayHost  string   `json:"displayHost"`
	Target       string   `json:"target"`
	AuthLabel    string   `json:"authLabel"`
	Healthy      bool     `json:"healthy"`
	LatencyMs    int64    `json:"latencyMs"`
	HealthKnown  bool     `json:"healthKnown"`
}

// ServerInput ekleme/düzenleme formundan gelen girdi.
type ServerInput struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	User         string   `json:"user"`
	AuthType     string   `json:"authType"`
	KeyPath      string   `json:"keyPath"`
	Password     string   `json:"password"`
	ExtraSSHArgs string   `json:"extraSSHArgs"`
	Notes        string   `json:"notes"`
	Tags         []string `json:"tags"`
}

type healthEntry struct {
	ok    bool
	lat   time.Duration
	known bool
}

// App Wails bağıntı (binding) katmanı.
type App struct {
	ctx context.Context
	st  *store.Store

	mu     sync.Mutex
	health map[string]healthEntry
}

func NewApp() *App {
	return &App{health: map[string]healthEntry{}}
}

// startup uygulama açıldığında çağrılır.
// jsLogPath arabağlantı (JS↔Go) olaylarını kaydeden hata ayıklama dosyası
// (platformun geçici dizini: Windows %TEMP%, Linux /tmp).
var jsLogPath = filepath.Join(os.TempDir(), "moor_gui_js.log")

func logJS(msg string) {
	line := time.Now().Format("2006-01-02 15:04:05.000") + " " + msg + "\n"
	if f, err := os.OpenFile(jsLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		f.WriteString(line)
		f.Close()
	}
}

// Log frontend'den gelen hata ayıklama mesajını dosyaya yazar.
func (a *App) Log(msg string) { logJS("js: " + msg) }

// Env ortam değişkeni değerini döndürür (yalnızca yerel hata ayıklama için).
func (a *App) Env(name string) string { return os.Getenv(name) }

// Platform çalışılan işletim sistemini döndürür (runtime.GOOS);
// frontend, platforma özgü komut/sözdizimi seçiminde kullanır.
func (a *App) Platform() string { return runtime.GOOS }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logJS("=== uygulama başlatılıyor ===")
	path, err := store.DefaultPath()
	if err != nil {
		path = filepath.Join(os.TempDir(), "moor", "servers.json")
	}
	st, err := store.Load(path)
	if err != nil {
		// Dosya bozuksa boş listeyle başla (kullanıcının verisini ezme).
		fmt.Fprintln(os.Stderr, "sunucu listesi okunamadı:", err)
		st = &store.Store{Path: path}
		logJS("hata: store yüklenemedi: " + err.Error())
	} else {
		logJS("store yüklendi: yol=" + path + " sunucu=" + fmt.Sprint(len(st.List())))
	}
	a.st = st
}

func (a *App) toView(s *model.Server) ServerView {
	a.mu.Lock()
	h, ok := a.health[s.ID]
	a.mu.Unlock()
	v := ServerView{
		ID:           s.ID,
		Name:         s.Name,
		Host:         s.Host,
		Port:         s.Port,
		User:         s.EffectiveUser(),
		AuthType:     string(s.AuthType),
		KeyPath:      s.KeyPath,
		HasPassword:  s.Password != "",
		ExtraSSHArgs: s.ExtraSSHArgs,
		Notes:        s.Notes,
		Tags:         s.Tags,
		SSHCommand:   sshx.SSHCommandLine(s),
		DisplayHost:  s.DisplayHost(),
		Target:       s.Target(),
		AuthLabel:    s.AuthLabel(),
	}
	if ok {
		v.HealthKnown = true
		v.Healthy = h.ok
		v.LatencyMs = h.lat.Milliseconds()
	}
	return v
}

// Servers tüm sunucuları (sağlık bilgisiyle) döndürür.
func (a *App) Servers() []ServerView {
	list := a.st.List()
	out := make([]ServerView, 0, len(list))
	for _, s := range list {
		out = append(out, a.toView(s))
	}
	return out
}

// SaveServer id boşsa ekler, doluysa günceller.
func (a *App) SaveServer(in ServerInput) error {
	logJS("SaveServer girdi: name=" + in.Name + " host=" + in.Host + " port=" + fmt.Sprint(in.Port) + " auth=" + in.AuthType + " id=" + in.ID)
	err := a.saveServer(in)
	if err != nil {
		logJS("SaveServer HATA: " + err.Error())
	} else {
		logJS("SaveServer tamam")
	}
	return err
}

func (a *App) saveServer(in ServerInput) error {
	sv := &model.Server{
		ID:           strings.TrimSpace(in.ID),
		Name:         strings.TrimSpace(in.Name),
		Host:         strings.TrimSpace(in.Host),
		Port:         in.Port,
		User:         strings.TrimSpace(in.User),
		KeyPath:      strings.TrimSpace(in.KeyPath),
		Password:     in.Password,
		ExtraSSHArgs: strings.TrimSpace(in.ExtraSSHArgs),
		Notes:        strings.TrimSpace(in.Notes),
		Tags:         cleanTags(in.Tags),
	}
	switch strings.ToLower(strings.TrimSpace(in.AuthType)) {
	case "key":
		sv.AuthType = model.AuthKey
	case "password":
		sv.AuthType = model.AuthPassword
	default:
		sv.AuthType = "" // Normalize belirler
	}
	sv.Normalize()
	if err := sv.Validate(); err != nil {
		return err
	}
	if in.ID == "" {
		a.st.Add(sv)
	} else {
		// Formda yer almayan senkron alanlarını mevcut kayıttan devral
		// (anahtar içeriği silinmesin).
		if old := a.st.FindByID(in.ID); old != nil {
			sv.KeyContent = old.KeyContent
			sv.KeyPassphrase = old.KeyPassphrase
		}
		a.st.Update(sv)
	}
	return a.st.Save()
}

// DeleteServer kimliğe göre siler.
func (a *App) DeleteServer(id string) error {
	a.st.Remove(id)
	a.mu.Lock()
	delete(a.health, id)
	a.mu.Unlock()
	return a.st.Save()
}

// ImportSSHConfig ~/.ssh/config'ten içe aktarır; kaç tane eklendiğini döndürür.
func (a *App) ImportSSHConfig() int {
	home, _ := os.UserHomeDir()
	entries, err := sshcfg.ParseFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		return 0
	}
	added := 0
	for _, sv := range sshcfg.ToServers(entries) {
		if _, i := a.st.FindByName(sv.Name); i >= 0 {
			continue
		}
		a.st.Add(sv)
		added++
	}
	if added > 0 {
		_ = a.st.Save()
	}
	return added
}

// JSONImportResult JSON içe aktarma sonucu.
type JSONImportResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
}

// ImportJSON bir yedek/metin dosyasındaki sunucuları ekler (aynı adlılar atlanır).
func (a *App) ImportJSON(text string) (JSONImportResult, error) {
	var in []*model.Server
	if err := json.Unmarshal([]byte(text), &in); err != nil {
		return JSONImportResult{}, fmt.Errorf("geçersiz JSON: %v", err)
	}
	res := JSONImportResult{}
	for _, sv := range in {
		sv.Normalize()
		if err := sv.Validate(); err != nil {
			continue
		}
		if _, i := a.st.FindByName(sv.Name); i >= 0 {
			res.Skipped++
			continue
		}
		a.st.Add(sv)
		res.Added++
	}
	if res.Added > 0 {
		if err := a.st.Save(); err != nil {
			return res, err
		}
	}
	return res, nil
}

// ExportJSON tüm sunucuların JSON metnini döndürür (yedek/aktarım için).
func (a *App) ExportJSON() string {
	data, err := json.MarshalIndent(a.st.List(), "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}

// PingSummary ping sonuç özeti.
type PingSummary struct {
	Online int `json:"online"`
	Total  int `json:"total"`
}

// PingAll tüm sunuculara paralel TCP sağlık kontrolü yapar (senkron bekler).
func (a *App) PingAll() PingSummary {
	servers := a.st.List()
	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(s *model.Server) {
			defer wg.Done()
			dur, err := sshx.TCPCheck(s.Host, s.Port, 4*time.Second)
			a.mu.Lock()
			if err != nil {
				a.health[s.ID] = healthEntry{ok: false, known: true}
			} else {
				a.health[s.ID] = healthEntry{ok: true, lat: dur, known: true}
			}
			a.mu.Unlock()
		}(s)
	}
	wg.Wait()
	sum := PingSummary{Total: len(servers)}
	for _, s := range servers {
		a.mu.Lock()
		h := a.health[s.ID]
		a.mu.Unlock()
		if h.known && h.ok {
			sum.Online++
		}
	}
	return sum
}

// SSHTestResult SSH test sonucu.
type SSHTestResult struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// TestSSH gerçek bir SSH oturum denemesi yapar (anahtar/şifre doğrulaması).
func (a *App) TestSSH(id string) SSHTestResult {
	s := a.st.FindByID(id)
	if s == nil {
		return SSHTestResult{Detail: "sunucu bulunamadı"}
	}
	res := sshx.SSHAuthCheck(s, 15*time.Second)
	if res.Err != nil {
		detail := strings.TrimSpace(res.Stderr)
		if detail == "" {
			detail = res.Err.Error()
		}
		return SSHTestResult{Detail: detail}
	}
	return SSHTestResult{OK: true, Detail: "SSH oturumu kuruldu"}
}

// GetServer düzenleme formu için tüm alanları (şifre dahil) döndürür.
func (a *App) GetServer(id string) ServerInput {
	s := a.st.FindByID(id)
	if s == nil {
		return ServerInput{}
	}
	return ServerInput{
		ID:           s.ID,
		Name:         s.Name,
		Host:         s.Host,
		Port:         s.Port,
		User:         s.User,
		AuthType:     string(s.AuthType),
		KeyPath:      s.KeyPath,
		Password:     s.Password,
		ExtraSSHArgs: s.ExtraSSHArgs,
		Notes:        s.Notes,
		Tags:         s.Tags,
	}
}

// OpenInTerminal sistem terminalinde (Konsole/x-terminal-emulator) ssh başlatır.
func (a *App) OpenInTerminal(id string) error {
	s := a.st.FindByID(id)
	if s == nil {
		return os.ErrNotExist
	}
	return openTerminal(sshx.SSHCommandLine(s))
}

// AccountView hesabın istemci tarafındaki durumu.
type AccountView struct {
	LoggedIn         bool   `json:"loggedIn"`
	Username         string `json:"username"`
	ServerURL        string `json:"serverUrl"`
	DefaultServerURL string `json:"defaultServerUrl"`
	LastSyncVersion  int64  `json:"lastSyncVersion"`
	LastSyncAt       string `json:"lastSyncAt"`
}

// SyncResult senkron turunun sonucu (frontend bildirimi için).
type SyncResult struct {
	OK          bool   `json:"ok"`
	Message     string `json:"message"`
	Version     int64  `json:"version"`
	LocalOnly   int    `json:"localOnly"`
	RemoteOnly  int    `json:"remoteOnly"`
	ChangedBoth int    `json:"changedBoth"`
}

func loadCreds() *syncx.Credentials {
	c, err := syncx.LoadCredentials()
	if err != nil {
		return &syncx.Credentials{}
	}
	return c
}

// AccountInfo senkron hesabının güncel durumunu döndürür.
func (a *App) AccountInfo() AccountView {
	c := loadCreds()
	v := AccountView{
		LoggedIn:         c.LoggedIn(),
		Username:         c.Username,
		ServerURL:        c.ServerURL,
		DefaultServerURL: syncx.DefaultBaseURL,
		LastSyncVersion:  c.LastSyncVersion,
	}
	if !c.LastSyncAt.IsZero() {
		v.LastSyncAt = c.LastSyncAt.Format("02/01/2006 15:04")
	}
	return v
}

// Login senkron hesabına giriş yapar (kayıtlı değilse hata döner).
func (a *App) Login(urlStr, username, password string) error {
	c, err := syncx.NewClient(urlStr)
	if err != nil {
		return err
	}
	if err := c.Login(username, password); err != nil {
		return err
	}
	creds := loadCreds()
	creds.ServerURL = strings.TrimRight(strings.TrimSpace(urlStr), "/")
	creds.Username = username
	creds.Token = c.Token()
	if err := creds.Save(); err != nil {
		return err
	}
	return nil
}

// Register yeni senkron hesabı oluşturur ve giriş yapar.
func (a *App) Register(urlStr, username, password string) error {
	c, err := syncx.NewClient(urlStr)
	if err != nil {
		return err
	}
	if err := c.Register(username, password); err != nil {
		return err
	}
	creds := loadCreds()
	creds.ServerURL = strings.TrimRight(strings.TrimSpace(urlStr), "/")
	creds.Username = username
	creds.Token = c.Token()
	if err := creds.Save(); err != nil {
		return err
	}
	return nil
}

// Logout senkron oturumunu kapatır.
func (a *App) Logout() error {
	c := loadCreds()
	if !c.LoggedIn() {
		return nil
	}
	if cl, err := syncx.NewClient(c.ServerURL); err == nil {
		cl.SetToken(c.Token)
		_ = cl.Logout()
	}
	c.Token = ""
	c.LastSyncVersion = 0
	return c.Save()
}

// ChangePassword senkron hesabının şifresini değiştirir (mevcut oturum
// korunur, diğer cihazlardaki oturumlar kapanır).
func (a *App) ChangePassword(oldPassword, newPassword string) error {
	c := loadCreds()
	if !c.LoggedIn() {
		return fmt.Errorf("önce giriş yapmalısınız")
	}
	cl, err := syncx.NewClient(c.ServerURL)
	if err != nil {
		return err
	}
	cl.SetToken(c.Token)
	return cl.ChangePassword(oldPassword, newPassword)
}

// DeleteAccount hesabı ve senkron verisini sunucudan tamamen siler.
func (a *App) DeleteAccount() error {
	c := loadCreds()
	if !c.LoggedIn() {
		return fmt.Errorf("önce giriş yapmalısınız")
	}
	cl, err := syncx.NewClient(c.ServerURL)
	if err != nil {
		return err
	}
	cl.SetToken(c.Token)
	if err := cl.DeleteAccount(); err != nil {
		return err
	}
	c.Token = ""
	c.LastSyncVersion = 0
	return c.Save()
}

// SyncNow tam bir senkronizasyon turu yapar ve depoyu günceller.
func (a *App) SyncNow() SyncResult {
	c := loadCreds()
	if !c.LoggedIn() {
		return SyncResult{OK: false, Message: "önce giriş yapmalısınız"}
	}
	cl, err := syncx.NewClient(c.ServerURL)
	if err != nil {
		return SyncResult{OK: false, Message: "geçersiz API adresi"}
	}
	cl.SetToken(c.Token)

	merged, sum, err := syncx.SyncStore(cl, a.st.List(), c.LastSyncVersion)
	if err != nil {
		return SyncResult{OK: false, Message: err.Error()}
	}
	a.st.ReplaceAll(merged)
	if err := a.st.Save(); err != nil {
		return SyncResult{OK: false, Message: "kaydedilemedi: " + err.Error()}
	}
	c.LastSyncVersion = sum.Version
	c.LastSyncAt = sum.At
	if err := c.Save(); err != nil {
		return SyncResult{OK: false, Message: "kimlik kaydedilemedi"}
	}
	msg := fmt.Sprintf("senkron tamam (sürüm %d)", sum.Version)
	if sum.LocalOnly > 0 || sum.RemoteOnly > 0 || sum.ChangedBoth > 0 {
		msg = fmt.Sprintf("senkron tamam: yerel %d, uzak %d, çakışma %d",
			sum.LocalOnly, sum.RemoteOnly, sum.ChangedBoth)
	}
	return SyncResult{OK: true, Message: msg, Version: sum.Version,
		LocalOnly: sum.LocalOnly, RemoteOnly: sum.RemoteOnly, ChangedBoth: sum.ChangedBoth}
}

func cleanTags(in []string) []string {
	var out []string
	for _, t := range in {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
