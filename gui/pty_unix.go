//go:build !windows

package gui

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// unixPty creack/pty master dosyasını ptyIO arayüzüne bağlar.
type unixPty struct {
	f   *os.File
	cmd *exec.Cmd
}

// startPty verilen komutu bir PTY içinde başlatır; oturum arayüzünü ve
// sürecin çıkışını bekleyen fonksiyonu döndürür.
func startPty(argv []string, cols, rows int) (ptyIO, func() int, error) {
	if _, err := exec.LookPath(argv[0]); err != nil {
		return nil, nil, fmt.Errorf("komut bulunamadı: %s", argv[0])
	}
	c := exec.Command(argv[0], argv[1:]...)
	// Gömülü terminal xterm-256color emüle eder (xterm.js). GUI masaüstünden
	// başlatıldığında ortamda TERM olmayabilir; ssh onu iletmez ve remote'daki
	// kabuk (fish vb.) uyarı verir. TERM'i garantiyle doğru değere sabitle.
	c.Env = withTerm(os.Environ(), "xterm-256color")
	f, err := pty.StartWithSize(c, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, nil, err
	}
	u := &unixPty{f: f, cmd: c}
	// done() yalnızca sürecin çıkışını bekler ve kodu döndürür; PTY'yi
	// KAPATMAZ. Kapatma, tüm çıktının termReader tarafından iletildiğinden
	// emin olunca (readerDone) exit goroutine'i tarafından u.Close() ile
	// yapılır; yoksa son veri parçası kaybolur (race).
	done := func() int {
		_ = c.Wait()
		code := 0
		if c.ProcessState != nil {
			code = c.ProcessState.ExitCode()
		}
		return code
	}
	return u, done, nil
}

func (u *unixPty) Read(p []byte) (int, error) { return u.f.Read(p) }
func (u *unixPty) Write(p []byte) (int, error) { return u.f.Write(p) }

func (u *unixPty) Resize(cols, rows int) error {
	return pty.Setsize(u.f, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
}

func (u *unixPty) Close() error {
	if u.cmd.Process != nil {
		_ = u.cmd.Process.Kill()
	}
	return u.f.Close()
}
