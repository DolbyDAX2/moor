// Package sshx ssh/scp/sftp/rsync komut satırlarını kurar ve çalıştırır.
package sshx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"moor/model"
)

// ExpandPath baştaki ~ sembolünü ana dizine genişletir.
func ExpandPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

// resolveKeyPath Windows'ta senkronlanmış KeyContent'i dosyaya aktarır;
// böylece başka OS'ten gelen KeyPath kullanılabilir. Diğer platformların
// mevcut KeyPath davranışı korunur.
func resolveKeyPath(s *model.Server) string {
	if s.AuthType != model.AuthKey {
		return ""
	}
	if content := strings.TrimSpace(s.KeyContent); runtime.GOOS == "windows" && content != "" {
		if p, err := materializeKey(content); err == nil {
			return p
		}
	}
	if strings.TrimSpace(s.KeyPath) != "" {
		return ExpandPath(s.KeyPath)
	}
	return ""
}

// materializeKey anahtar içeriğini 0600 izinli, içerik hash'i adlı geçici bir
// dosyaya yazar ve yolunu döndürür. Dosya zaten aynı içerikle varsa yeniden
// yazılmaz. Dizin 0700 (sadece sahip) olur.
func materializeKey(content string) (string, error) {
	sum := sha256.Sum256([]byte(content))
	name := "moor-key-" + hex.EncodeToString(sum[:8]) + ".pem"
	dir := filepath.Join(os.TempDir(), "moor-keys")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return path, nil // zaten aynı içerik
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// SSHArgs sunucu için ortak ssh seçeneklerini döndürür.
func SSHArgs(s *model.Server) []string {
	args := []string{"-p", strconv.Itoa(s.Port), "-o", "StrictHostKeyChecking=accept-new"}
	if keyPath := resolveKeyPath(s); keyPath != "" {
		args = append(args, "-i", keyPath)
	}
	args = append(args, strings.Fields(s.ExtraSSHArgs)...)
	return args
}

// execPrefix şifreli sunucular için sshpass öneki ekler.
func execPrefix(s *model.Server, program string) []string {
	if s.Password != "" {
		if path, err := exec.LookPath("sshpass"); err == nil {
			return []string{path, "-p", s.Password, program}
		}
	}
	return []string{program}
}

// SSHFullCommand etkileşimli ssh oturumu için tam komut satırını döndürür.
func SSHFullCommand(s *model.Server) []string {
	return append(execPrefix(s, "ssh"), append(SSHArgs(s), s.Target())...)
}

// SSHCommandLine komutu (şifre öneği olmadan) metin olarak döndürür;
// pano'ya kopyalama gibi amaçlar içindir.
func SSHCommandLine(s *model.Server) string {
	return strings.Join(append([]string{"ssh"}, append(SSHArgs(s), s.Target())...), " ")
}

// SFTPFullCommand etkileşimli sftp istemcisi için tam komut satırını döndürür.
func SFTPFullCommand(s *model.Server) []string {
	return append(execPrefix(s, "sftp"), append(SSHArgs(s), s.Target())...)
}

// RemoteCommandFull sunucuda bir kabuk komutu çalıştırmak için argv döndürür.
func RemoteCommandFull(s *model.Server, cmdText string) []string {
	return append(execPrefix(s, "ssh"), append(SSHArgs(s), s.Target(), cmdText)...)
}

// SCPFullCommand scp komut satırını döndürür.
func SCPFullCommand(s *model.Server, src, dst string) []string {
	args := []string{"-P", strconv.Itoa(s.Port)}
	if keyPath := resolveKeyPath(s); keyPath != "" {
		args = append(args, "-i", keyPath)
	}
	full := append([]string{}, execPrefix(s, "scp")...)
	full = append(full, args...)
	return append(full, src, dst)
}

// RsyncFullCommand ssh üzerinden rsync komut satırını döndürür.
func RsyncFullCommand(s *model.Server, src, dst string) []string {
	sshLine := strings.Join(append([]string{"ssh"}, SSHArgs(s)...), " ")
	args := []string{"-avz", "-e", sshLine}
	full := append([]string{}, execPrefix(s, "rsync")...)
	full = append(full, args...)
	return append(full, src, dst)
}

// ForwardFullCommand port yönlendirme için ssh komut satırını döndürür.
func ForwardFullCommand(s *model.Server, localPort int, remoteHost string, remotePort int) []string {
	spec := fmt.Sprintf("%d:%s:%d", localPort, remoteHost, remotePort)
	full := append([]string{}, execPrefix(s, "ssh")...)
	full = append(full, "-N", "-L", spec)
	full = append(full, SSHArgs(s)...)
	return append(full, s.Target())
}

// RunInteractive komut satırını mevcut terminalde çalıştırır (CLI modu).
func RunInteractive(argv ...string) error {
	c := exec.Command(argv[0], argv[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// CapturedResult yakalanan bir komut çalıştırmasının sonucudur.
type CapturedResult struct {
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error
}

// RunCaptured komutu zaman aşımıyla çalıştırıp çıktıyı yakalar.
func RunCaptured(timeout time.Duration, argv ...string) CapturedResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out, errb bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errb
	start := time.Now()
	err := c.Run()
	res := CapturedResult{Stdout: out.String(), Stderr: errb.String(), Duration: time.Since(start)}
	if ctx.Err() == context.DeadlineExceeded {
		res.Err = fmt.Errorf("zaman aşımı (%s)", timeout.Round(time.Second))
	} else if err != nil {
		res.Err = err
	}
	return res
}

// TCPCheck host:port'a TCP bağlantısı dener, gecikmeyi döndürür.
func TCPCheck(host string, port int, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), timeout)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start), nil
}

// SSHAuthCheck anahtar/şifre ile gerçekten oturum açılıp açılamadığını dener.
func SSHAuthCheck(s *model.Server, timeout time.Duration) CapturedResult {
	base := append([]string{}, SSHArgs(s)...)
	if s.Password != "" {
		if path, err := exec.LookPath("sshpass"); err == nil {
			argv := append(append([]string{path, "-p", s.Password, "ssh"}, base...), s.Target(), "true")
			return RunCaptured(timeout, argv...)
		}
	}
	args := append([]string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=5"}, base...)
	argv := append(append([]string{"ssh"}, args...), s.Target(), "true")
	return RunCaptured(timeout, argv...)
}
