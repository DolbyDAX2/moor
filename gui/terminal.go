package gui

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/creack/pty"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"moor/sshx"
)

// termSession bir PTY içine başlatılmış canlı oturumdur.
type termSession struct {
	id   string
	cmd  *exec.Cmd
	ptmx *os.File
}

var (
	termsMu sync.Mutex
	terms   = map[string]*termSession{}
)

func newSessID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// withTerm, env'in TERM değerini term'e sabitler (yoksa ekler, varsa değiştirir).
func withTerm(env []string, term string) []string {
	key := "TERM="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, key) {
			out = append(out, key+term)
			found = true
		} else {
			out = append(out, e)
		}
	}
	if !found {
		out = append(out, key+term)
	}
	return out
}

// utf8ValidEnd, b'nin sondan en fazla 3 bayta kadar geriye bakarak,
// sonundaki yarım kalmış UTF-8 dizgesini ayırır ve geçerli önekin uzunluğunu döndürür.
func utf8ValidEnd(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	if utf8.FullRune(b) {
		return len(b)
	}
	for i := len(b) - 1; i >= 0 && i > len(b)-4; i-- {
		if utf8.RuneStart(b[i]) {
			return i
		}
	}
	return 0
}

// startPTY verilen komutu bir PTY içinde başlatır ve oturum kimliği döndürür.
func (a *App) startPTY(argv []string, cols, rows int) (string, error) {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return "", fmt.Errorf("komut bulunamadı: %s", argv[0])
	}
	c := exec.Command(argv[0], argv[1:]...)
	// Gömülü terminal xterm-256color emüle eder (xterm.js). GUI masaüstünden
	// başlatıldığında ortamda TERM olmayabilir; ssh onu iletmez ve remote'daki
	// kabuk (fish vb.) uyarı verir. TERM'i garantiyle doğru değere sabitle.
	c.Env = withTerm(os.Environ(), "xterm-256color")
	f, err := pty.StartWithSize(c, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return "", err
	}
	id := newSessID()
	termsMu.Lock()
	terms[id] = &termSession{id: id, cmd: c, ptmx: f}
	termsMu.Unlock()

	go a.termReader(id)
	go func() {
		_ = c.Wait()
		code := 0
		if c.ProcessState != nil {
			code = c.ProcessState.ExitCode()
		}
		wailsruntime.EventsEmit(a.ctx, "term:exit:"+id, map[string]any{"code": code})
		termsMu.Lock()
		delete(terms, id)
		termsMu.Unlock()
	}()
	return id, nil
}

// termReader PTY çıktısını okur ve (UTF-8 sınır güvenli) parçalar halinde öne iletilir.
func (a *App) termReader(id string) {
	termsMu.Lock()
	s := terms[id]
	termsMu.Unlock()
	if s == nil {
		return
	}
	buf := make([]byte, 16384)
	var left []byte
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			data := make([]byte, len(left)+n)
			copy(data, left)
			copy(data[len(left):], buf[:n])
			end := utf8ValidEnd(data)
			if end > 0 {
				wailsruntime.EventsEmit(a.ctx, "term:data:"+id, string(data[:end]))
				rest := make([]byte, len(data)-end)
				copy(rest, data[end:])
				left = rest
			} else {
				left = append([]byte{}, data...)
			}
		}
		if err != nil {
			return
		}
	}
}

// StartSSHTerminal gömülü terminal için etkileşimli bir SSH oturumu başlatır.
// Şifreli sunucularda sshpass KULLANILMAZ: şifre istemi xterm içinde
// doğrudan kullanıcıya sorulur (gerçek terminal deneyimi).
func (a *App) StartSSHTerminal(serverID string, cols, rows int) (string, error) {
	s := a.st.FindByID(serverID)
	if s == nil {
		return "", errors.New("sunucu bulunamadı")
	}
	argv := append([]string{"ssh"}, append(sshx.SSHArgs(s), s.Target())...)
	return a.startPTY(argv, cols, rows)
}

// StartLocalShell yerel bir kabuk (veya cmdText verilirse o komutu) PTY içinde başlatır.
func (a *App) StartLocalShell(cols, rows int, cmdText string) (string, error) {
	if cmdText != "" {
		return a.startPTY([]string{"/bin/sh", "-c", cmdText}, cols, rows)
	}
	sh := "/bin/sh"
	if p, err := exec.LookPath("bash"); err == nil {
		sh = p
	}
	return a.startPTY([]string{sh, "-l"}, cols, rows)
}

// WriteTerminal oturuma kullanıcı girdisi yazar.
func (a *App) WriteTerminal(sessID, data string) error {
	termsMu.Lock()
	s := terms[sessID]
	termsMu.Unlock()
	if s == nil {
		return errors.New("oturum bulunamadı")
	}
	_, err := s.ptmx.Write([]byte(data))
	return err
}

// ResizeTerminal oturumun PTY boyutunu günceller.
func (a *App) ResizeTerminal(sessID string, cols, rows int) error {
	termsMu.Lock()
	s := terms[sessID]
	termsMu.Unlock()
	if s == nil {
		return errors.New("oturum bulunamadı")
	}
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

// CloseTerminal oturumu kapatır ve süreci sonlandırır.
func (a *App) CloseTerminal(sessID string) error {
	termsMu.Lock()
	s := terms[sessID]
	delete(terms, sessID)
	termsMu.Unlock()
	if s == nil {
		return nil
	}
	_ = s.ptmx.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}
