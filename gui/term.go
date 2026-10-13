package gui

import (
	"errors"
	"os/exec"
	"runtime"
)

var errNoTerminal = errors.New("uygun bir terminal emülatörü bulunamadı")

// openTerminal verilen komutu sistemin uygun terminal emülatöründe yeni
// pencerede çalıştırır. Linux'ta KDE'de Konsole, ardından gnome-terminal,
// x-terminal-emulator ve xterm denenir; Windows'ta önce Windows Terminal
// (wt), yoksa klasik cmd penceresi (start) kullanılır.
func openTerminal(cmd string) error {
	if runtime.GOOS == "windows" {
		return openTerminalWindows(cmd)
	}
	candidates := []struct {
		bin  string
		args []string
	}{
		{"konsole", []string{"--hold", "-e"}},
		{"gnome-terminal", []string{"--"}},
		{"x-terminal-emulator", []string{"-e"}},
		{"xterm", []string{"-e"}},
	}
	for _, c := range candidates {
		path, err := exec.LookPath(c.bin)
		if err != nil {
			continue
		}
		argv := append(append([]string{}, path), c.args...)
		argv = append(argv, "sh", "-c", cmd)
		return exec.Command(argv[0], argv[1:]...).Start()
	}
	return errNoTerminal
}

// openTerminalWindows komutu Windows'ta yeni bir terminal penceresinde çalıştırır.
// Tercihen Windows Terminal (wt) kullanılır; yoksa "start" ile yeni bir
// cmd penceresi açılır (komut bitince pencere kapanmasın diye /k verilir).
func openTerminalWindows(cmd string) error {
	if wt, err := exec.LookPath("wt"); err == nil {
		return exec.Command(wt, "cmd", "/c", cmd).Start()
	}
	if cmdExe, err := exec.LookPath("cmd"); err == nil {
		return exec.Command(cmdExe, "/c", "start", "Moor", "cmd", "/k", cmd).Start()
	}
	return errNoTerminal
}
