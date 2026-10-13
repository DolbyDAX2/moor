//go:build windows

package gui

import (
	"os"
	"strings"
	"testing"
	"time"

	"moor/sshx"
	"moor/store"
)

// Explicit opt-in: uses a configured SSH server, never runs in normal tests.
func TestWindowsSSHPTY(t *testing.T) {
	name := os.Getenv("MOOR_TEST_SSH_SERVER")
	if name == "" {
		t.Skip("set MOOR_TEST_SSH_SERVER to opt in")
	}
	path, err := store.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	server, _ := st.FindByName(name)
	if server == nil {
		t.Fatal("server not found")
	}
	argv := append([]string{"ssh"}, append(sshx.SSHArgs(server), server.Target())...)
	p, wait, err := startPty(argv, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	chunks := make(chan string, 32)
	go func() {
		b := make([]byte, 16384)
		for {
			n, err := p.Read(b)
			if n > 0 {
				chunks <- string(b[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	exited := make(chan int, 1)
	go func() { exited <- wait() }()
	if err := p.Resize(100, 30); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if _, err := p.Write([]byte("printf 'MOOR_%s\\n' SSH_PTY_OK\r")); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(15 * time.Second)
	var output strings.Builder
	for {
		select {
		case s := <-chunks:
			output.WriteString(s)
			if strings.Contains(output.String(), "MOOR_SSH_PTY_OK") {
				t.Log("SSH PTY marker received")
				return
			}
		case code := <-exited:
			t.Fatalf("SSH exited: 0x%X; output=%q", code, output.String())
		case <-deadline:
			t.Fatalf("SSH PTY timeout; output=%q", output.String())
		}
	}
}
