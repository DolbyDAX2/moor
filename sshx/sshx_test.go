package sshx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"moor/model"
)

func TestSSHArgsKey(t *testing.T) {
	s := &model.Server{Name: "a", Host: "h", Port: 2222, User: "root", AuthType: model.AuthKey, KeyPath: "~/.ssh/k.pem"}
	joined := strings.Join(SSHArgs(s), " ")
	for _, want := range []string{"-p 2222", "-i " + homeJoin(t, ".ssh", "k.pem"), "StrictHostKeyChecking=accept-new"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("eksik: %q (tamamı: %s)", want, joined)
		}
	}
}

func TestSSHFullCommandPassword(t *testing.T) {
	s := &model.Server{Name: "a", Host: "h", Port: 22, User: "root", AuthType: model.AuthPassword, Password: "pw"}
	full := SSHFullCommand(s)
	if path, err := exec.LookPath("sshpass"); err == nil {
		if full[0] != path || full[1] != "-p" || full[2] != "pw" || full[3] != "ssh" {
			t.Fatalf("sshpass öneki hatalı: %v", full)
		}
	} else if full[0] != "ssh" {
		t.Fatalf("düz ssh bekleniyor: %v", full)
	}
	if last := full[len(full)-1]; last != "root@h" {
		t.Fatalf("hedef yanlış: %v", full)
	}
}

func TestForwardCommand(t *testing.T) {
	s := &model.Server{Name: "a", Host: "h", Port: 22, User: "root"}
	joined := strings.Join(ForwardFullCommand(s, 5432, "127.0.0.1", 5432), " ")
	for _, want := range []string{"-N", "-L 5432:127.0.0.1:5432"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("eksik: %q (tamamı: %s)", want, joined)
		}
	}
}

func TestExpandPath(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME tanımlı değil")
	}
	if got := ExpandPath("~/a/b"); got != filepath.Join(home, "a", "b") {
		t.Fatalf("~ genişletilmedi: %s", got)
	}
	if got := ExpandPath("a/b"); got != "a/b" {
		t.Fatalf("değişmez kalmalıydı: %s", got)
	}
}

func homeJoin(t *testing.T, parts ...string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(append([]string{home}, parts...)...)
}
