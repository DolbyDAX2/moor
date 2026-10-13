package gui

// ptyIO platforma özgü sahte terminal (PTY/ConPTY) oturumunun arayüzü.
// Unix'te creack/pty master dosyasını sarar (pty_unix.go); Windows'ta
// ConPTY API'sini doğrudan çağırır (pty_windows.go — creack/pty
// Windows'u desteklemediği için).
type ptyIO interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	// Resize pencere boyutunu sütun/satır olarak günceller.
	Resize(cols, rows int) error
	// Close oturumu sonlandırır (süreç de dahil) ve kaynakları serbest bırakır.
	Close() error
}
