package gui

import (
	"errors"
	"os/exec"
)

var errNoTerminal = errors.New("uygun bir terminal emülatörü bulunamadı")

// openTerminal verilen komutu sistemin uygun terminal emülatöründe yeni pencerede çalıştırır.
// KDE'de Konsole, ardından gnome-terminal, x-terminal-emulator ve xterm denenir.
func openTerminal(cmd string) error {
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
