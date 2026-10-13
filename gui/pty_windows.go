//go:build windows

// Bu dosya Windows'ta gömülü terminal için ConPTY (Windows 10 1607+
// yerel sahte konsol) katmanını saf Go ile uygular. creack/pty
// Windows'u desteklemediği için ConPTY API'si golang.org/x/sys/windows
// üzerinden çağrılır (cgo gerekmez).
//
// Akış:
//   - Giriş ve çıkış için ayrı overlapped pipe'lar kurulur.
//   - CreatePseudoConsole pipe uçlarını sahte konsola bağlar.
//   - CreateProcessW, PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE özniteliğiyle
//     (x/sys'in NewProcThreadAttributeList/Update ile) süreci ConPTY
//     içinde başlatır.
//   - Okuma/yazma, işlem başına olay (event) ile overlapped
//     ReadFile/WriteFile yapılır; I/O sırasında kilit tutulmaz ki
//     Close() asla I/O'ya takılmasın.

package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ConPTY API'si Windows 10 2004 (build 19041)+ sürümlerde kernelbase.dll
// içinde yaşar; önceki sürümlerde api-ms-win-conpty-l1-1-0.dll
// forwarder'ındaydı. Her ikisini de deneyelim.
var (
	conptyCreate          *windows.LazyProc
	conptyResize          *windows.LazyProc
	conptyClose           *windows.LazyProc
	ntCreateFileProc      *windows.LazyProc
	ntCreateNamedPipeProc *windows.LazyProc
)

func init() {
	for _, name := range []string{"kernelbase.dll", "api-ms-win-conpty-l1-1-0.dll"} {
		dll := windows.NewLazySystemDLL(name)
		pCreate := dll.NewProc("CreatePseudoConsole")
		if err := pCreate.Find(); err == nil {
			conptyCreate = pCreate
			conptyResize = dll.NewProc("ResizePseudoConsole")
			conptyClose = dll.NewProc("ClosePseudoConsole")
			break
		}
	}
	// DUPLEX overlapped pipe için Nt* API'leri (Windows Terminal'ın yöntemi).
	ntCreateFileProc = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtCreateFile")
	ntCreateNamedPipeProc = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtCreateNamedPipeFile")
}

// conptySession bir ConPTY'ye bağlı canlı oturumdur.
type conptySession struct {
	mu       sync.Mutex // yalnızca kapanış/teardown durumunu serialize eder
	pc       windows.Handle
	proc     windows.Handle // yalnızca done() kapatır
	in       windows.Handle // giriş pipe'ının server ucu (biz yazarız)
	out      windows.Handle // çıkış pipe'ının server ucu (biz okuruz)
	tornDown bool
	exited   bool
	exitCode int
}

// startupInfoEx = STARTUPINFOEXW (STARTUPINFO + öznitelik listesi işaretçisi).
type startupInfoEx struct {
	windows.StartupInfo
	lpAttributeList *windows.ProcThreadAttributeList
}

// quoteArg MSVC komut satırı sözdizimine göre argümanı tırnaklar.
func quoteArg(s string) string {
	if s != "" && !strings.ContainsAny(s, " \t\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		bs := 0
		for i < len(s) && s[i] == '\\' {
			bs++
			i++
		}
		if i == len(s) {
			b.WriteString(strings.Repeat("\\", bs*2+1))
			break
		}
		if s[i] == '"' {
			b.WriteString(strings.Repeat("\\", bs*2+1))
			b.WriteByte('"')
		} else {
			b.WriteString(strings.Repeat("\\", bs))
			b.WriteByte(s[i])
		}
		i++
	}
	b.WriteByte('"')
	return b.String()
}

// envBlock ortam değişkenlerini CreateProcessW'nin istediği UTF-16
// bloğa çevirir: her "A=B" girdisi NUL ile biter, blok çift NUL ile.
// UTF16FromString her girdinin sonlandırıcı NUL'unu zaten ekler.
func envBlock(env []string) []uint16 {
	var buf []uint16
	for _, kv := range env {
		u, err := windows.UTF16FromString(kv)
		if err != nil {
			continue
		}
		buf = append(buf, u...)
	}
	if len(buf) == 0 {
		buf = append(buf, 0)
	}
	buf = append(buf, 0) // blok sonu: çift NUL
	return buf
}

// coordPacked, COORD yapısını x64 Windows çağrı sözleşmesine göre tek
// register değerine paketler (X: alt 16 bit, Y: üst 16 bit).
func coordPacked(cols, rows int) uintptr {
	return uintptr(uint16(cols)) | uintptr(uint16(rows))<<16
}

// overlappedRW overlapped ReadFile/WriteFile çağrısını senkron biçimde tamamlar.
func overlappedRW(fn func() error, handle windows.Handle, ov *windows.Overlapped, done *uint32) error {
	err := fn()
	if err == windows.ERROR_IO_PENDING {
		windows.WaitForSingleObject(ov.HEvent, windows.INFINITE)
		err = windows.GetOverlappedResult(handle, ov, done, false)
	}
	return err
}

// --- Nt* yapıları ve DUPLEX pipe (Windows Terminal'ın yöntemi) ---
//
// Her akış için ayrı bir DUPLEX overlapped pipe oluşturulur. Aynı client
// handle'ı hem ConPTY girişine hem çıkışına vermek etkileşimli SSH'yi bozabilir.
// Bu yardımcı CreateOverlappedPipe'ın NtCreateNamedPipeFile eşleniğidir.

type ntUnicodeString struct {
	Length        uint16
	MaximumLength uint16
	Buffer        *uint16
}

type ntObjectAttributes struct {
	Length                   uint32
	RootDirectory            windows.Handle
	ObjectName               *ntUnicodeString
	Attributes               uint32
	SecurityDescriptor       uintptr
	SecurityQualityOfService uintptr
}

type ntIOStatusBlock struct {
	Status      uintptr
	Information uintptr
}

type ntLargeInteger struct {
	QuadPart int64
}

const (
	ntGenericRead        = 0x80000000
	ntGenericWrite       = 0x40000000
	ntSynchronize        = 0x00100000
	ntFileShareRead      = 0x00000001
	ntFileShareWrite     = 0x00000002
	ntFileOpen           = 0x00000001
	ntFileCreate         = 0x00000002
	ntFileNonDirectory   = 0x00000040
	ntFileSyncNonAlert   = 0x00000020
	ntObjCaseInsensitive = 0x00000040
	ntPipeByteStreamType = 0x00000003
	ntPipeByteStreamMode = 0x00000000
	ntPipeQueueOperation = 0x00000000
)

// createDuplexPipe, Windows Terminal'ın yaptığı gibi çift yönlü (DUPLEX)
// overlapped bir isimsiz pipe oluşturur ve (server, client) uçlarını döndürür.
func createDuplexPipe(bufferSize uint32) (server, client windows.Handle, err error) {
	if ntCreateFileProc == nil || ntCreateNamedPipeProc == nil {
		return 0, 0, fmt.Errorf("Nt* pipe API'leri bulunamadı")
	}
	// 1) \\Device\\NamedPipe\\ dizinini aç.
	path16, perr := windows.UTF16FromString(`\Device\NamedPipe\`)
	if perr != nil {
		return 0, 0, fmt.Errorf("dizin yolu kodlaması: %w", perr)
	}
	pathUs := &ntUnicodeString{
		Length:        uint16((len(path16) - 1) * 2),
		MaximumLength: uint16(len(path16) * 2),
		Buffer:        &path16[0],
	}
	dirOa := &ntObjectAttributes{
		Length:     uint32(unsafe.Sizeof(ntObjectAttributes{})),
		ObjectName: pathUs,
	}
	var iosb ntIOStatusBlock
	var dir windows.Handle
	r, _, callErr := ntCreateFileProc.Call(
		uintptr(unsafe.Pointer(&dir)),
		uintptr(ntSynchronize|ntGenericRead),
		uintptr(unsafe.Pointer(dirOa)),
		uintptr(unsafe.Pointer(&iosb)),
		0, // AllocationSize
		0, // FileAttributes
		uintptr(ntFileShareRead|ntFileShareWrite),
		uintptr(ntFileOpen),
		uintptr(ntFileSyncNonAlert),
		0, // EaBuffer
		0, // EaLength
	)
	if r != 0 {
		return 0, 0, fmt.Errorf("NtCreateFile(dizin): NTSTATUS=0x%X (%v)", r, callErr)
	}
	defer windows.CloseHandle(dir)

	// 2) DUPLEX pipe'ı oluştur (server ucu). Boş UNICODE_STRING = isimsiz pipe.
	emptyUs := &ntUnicodeString{}
	srvOa := &ntObjectAttributes{
		Length:        uint32(unsafe.Sizeof(ntObjectAttributes{})),
		RootDirectory: dir,
		ObjectName:    emptyUs,
		Attributes:    ntObjCaseInsensitive,
	}
	var timeout ntLargeInteger
	timeout.QuadPart = -100000000 // ~1 sn (100ns birim, negatif = süre)
	r, _, callErr = ntCreateNamedPipeProc.Call(
		uintptr(unsafe.Pointer(&server)),
		uintptr(ntSynchronize|ntGenericRead|ntGenericWrite),
		uintptr(unsafe.Pointer(srvOa)),
		uintptr(unsafe.Pointer(&iosb)),
		uintptr(ntFileShareRead|ntFileShareWrite),
		uintptr(ntFileCreate),
		0, // CreateOptions (0 = overlapped/queued)
		uintptr(ntPipeByteStreamType),
		uintptr(ntPipeByteStreamMode),
		uintptr(ntPipeQueueOperation),
		1,                   // MaximumInstances
		uintptr(bufferSize), // InboundQuota
		uintptr(bufferSize), // OutboundQuota
		uintptr(unsafe.Pointer(&timeout)),
	)
	if r != 0 {
		return 0, 0, fmt.Errorf("NtCreateNamedPipeFile: NTSTATUS=0x%X (%v)", r, callErr)
	}

	// 3) client ucunu aç (server handle'ı RootDirectory olarak).
	cliOa := &ntObjectAttributes{
		Length:        uint32(unsafe.Sizeof(ntObjectAttributes{})),
		RootDirectory: server,
		ObjectName:    emptyUs,
		Attributes:    ntObjCaseInsensitive,
	}
	r, _, callErr = ntCreateFileProc.Call(
		uintptr(unsafe.Pointer(&client)),
		uintptr(ntSynchronize|ntGenericRead|ntGenericWrite),
		uintptr(unsafe.Pointer(cliOa)),
		uintptr(unsafe.Pointer(&iosb)),
		0,
		0,
		uintptr(ntFileShareRead|ntFileShareWrite),
		uintptr(ntFileOpen),
		uintptr(ntFileNonDirectory),
		0,
		0,
	)
	if r != 0 {
		windows.CloseHandle(server)
		return 0, 0, fmt.Errorf("NtCreateFile(client): NTSTATUS=0x%X (%v)", r, callErr)
	}
	return server, client, nil
}

// startPty verilen komutu bir ConPTY içinde başlatır.
func startPty(argv []string, cols, rows int) (ptyIO, func() int, error) {
	if conptyCreate == nil {
		return nil, nil, fmt.Errorf("ConPTY bu Windows sürümünde kullanılamıyor (Windows 10 1607+ gerekli)")
	}
	// Git/MSYS ssh.exe yerel ConPTY ile uyumlu olmayabilir. Windows'un
	// OpenSSH istemcisini tercih et; yoksa PATH çözümlemesini koru.
	program := argv[0]
	if strings.EqualFold(program, "ssh") || strings.EqualFold(program, "ssh.exe") {
		native := filepath.Join(os.Getenv("SystemRoot"), "System32", "OpenSSH", "ssh.exe")
		if info, err := os.Stat(native); err == nil && !info.IsDir() {
			program = native
		}
	}
	exe, err := exec.LookPath(program)
	if err != nil {
		return nil, nil, fmt.Errorf("komut bulunamadı: %s", argv[0])
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	input, inputClient, err := createDuplexPipe(128 * 1024)
	if err != nil {
		return nil, nil, err
	}
	output, outputClient, err := createDuplexPipe(128 * 1024)
	if err != nil {
		windows.CloseHandle(input)
		windows.CloseHandle(inputClient)
		return nil, nil, err
	}

	// 2) ConPTY oluştur. CreatePseudoConsole HRESULT döndürür (S_OK = 0);
	//    COORD (4 bayt) x64'ta tek register olarak paketlenir.
	//    Giriş ve çıkış ayrı pipe uçlarına bağlanır.
	var pc windows.Handle
	r, _, callErr := conptyCreate.Call(
		coordPacked(cols, rows),
		uintptr(inputClient), uintptr(outputClient),
		0,
		uintptr(unsafe.Pointer(&pc)),
	)
	windows.CloseHandle(inputClient)
	windows.CloseHandle(outputClient)
	cleanupPipes := func() {
		windows.CloseHandle(input)
		windows.CloseHandle(output)
	}
	if r != 0 {
		cleanupPipes()
		return nil, nil, fmt.Errorf("CreatePseudoConsole: HRESULT=0x%X (%v)", r, callErr)
	}

	// 3) Öznitelik listesi: x/sys'in konteyneri, Windows'un kendi
	// InitializeProcThreadAttributeList/UpdateProcThreadAttribute API'lerini
	// kullandığı için yerleşim garantili doğrudur.
	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		conptyClose.Call(uintptr(pc))
		cleanupPipes()
		return nil, nil, fmt.Errorf("öznitelik listesi: %w", err)
	}
	defer attrList.Delete()
	// ÖNEMLİ: PSEUDOCONSOLE özniteliğinin değeri HPCON handle'ının KENDİSİdir
	// (gösterici değil). CreateProcessW bu değeri doğrudan sahte konsol handle'
	// olarak kullanır; &pc vermek geçersiz bir adres olur ve çocuk süreç
	// 0xC0000142 (STATUS_DLL_INIT_FAILED) ile ölür. Windows Terminal da
	// _hPC.get() (handle değerini) doğrudan geçirir.
	// UpdateProcThreadAttribute doğrudan çağrılır ki handle degeri saf
	// uintptr olarak geçsin (x/sys'in Update'ı unsafe.Pointer ister).
	updateAttr := windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")
	ok, _, uerr := updateAttr.Call(
		uintptr(unsafe.Pointer(attrList.List())),
		0,
		uintptr(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE),
		uintptr(pc), // handle degeri dogrudan (gostergici DEGIL)
		8,           // sizeof(HANDLE)
		0,
		0,
	)
	if ok == 0 {
		conptyClose.Call(uintptr(pc))
		cleanupPipes()
		return nil, nil, fmt.Errorf("PSEUDOCONSOLE özniteliği: %w", uerr)
	}

	si := &startupInfoEx{}
	si.Cb = uint32(unsafe.Sizeof(startupInfoEx{}))
	// Windows Terminal ile birebir aynı: STARTF_USESTDHANDLES (hStd* = NULL).
	// Bu flag olmadan çocuk süreç, konsolsuz GUI ebeveynin std handle'larını
	// miras alır ve DLL init sırasında 0xC0000142 (STATUS_DLL_INIT_FAILED) verir.
	si.Flags = windows.STARTF_USESTDHANDLES
	si.lpAttributeList = attrList.List()

	cmdLine := make([]string, len(argv))
	cmdLine[0] = quoteArg(exe)
	for i := 1; i < len(argv); i++ {
		cmdLine[i] = quoteArg(argv[i])
	}
	// cmd /c <metin>: metin bütünü alıntılanmaz; cmd.exe & | % gibi
	// karakterleri kendisi yorumlasın (yerel kabuk komutları için).
	if len(argv) == 3 && strings.EqualFold(filepath.Base(argv[0]), "cmd") && argv[1] == "/c" {
		cmdLine[2] = argv[2]
	}
	cmd16, err := windows.UTF16FromString(strings.Join(cmdLine, " "))
	if err != nil {
		conptyClose.Call(uintptr(pc))
		cleanupPipes()
		return nil, nil, fmt.Errorf("komut satırı kodlaması: %w", err)
	}
	exe16, _ := windows.UTF16FromString(exe)
	env := envBlock(withTerm(os.Environ(), "xterm-256color"))

	var pi windows.ProcessInformation
	createProc := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreateProcessW")
	r, _, callErr = createProc.Call(
		uintptr(unsafe.Pointer(&exe16[0])),
		uintptr(unsafe.Pointer(&cmd16[0])),
		0, // süreç öznitelikleri
		0, // iş parçacığı öznitelikleri
		0, // bInheritHandles = FALSE
		windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT,
		uintptr(unsafe.Pointer(&env[0])),
		0, // çalışma dizini (miras)
		uintptr(unsafe.Pointer(si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r == 0 {
		conptyClose.Call(uintptr(pc))
		cleanupPipes()
		return nil, nil, fmt.Errorf("CreateProcessW (%s): %w", argv[0], callErr)
	}
	windows.CloseHandle(pi.Thread) // ConPTY'de iş parçacığı beklenmez

	// Her pipe'ın server ucu uygulamada, client ucu ConPTY'de tutulur.
	sess := &conptySession{
		pc:   pc,
		proc: pi.Process,
		in:   input,
		out:  output,
	}
	// done() yalnızca sürecin çıkışını bekler ve kodu döndürür; pipe/PTY'yi
	// KAPATMAZ. Kapatma, tüm çıktının termReader tarafından iletildiğinden
	// emin olunca (readerDone) exit goroutine'i tarafından s.io.Close() ile
	// yapılır; yoksa son veri parçası kaybolur (race).
	done := func() int {
		windows.WaitForSingleObject(sess.proc, windows.INFINITE)
		var code uint32
		_ = windows.GetExitCodeProcess(sess.proc, &code)
		sess.mu.Lock()
		windows.CloseHandle(sess.proc)
		sess.proc = 0
		sess.exitCode = int(code)
		sess.exited = true
		sess.mu.Unlock()
		return int(code)
	}
	return sess, done, nil
}

// withEvent işlem başına bir olayla overlapped I/O yapar (I/O sırasında
// oturum kilidi tutulmaz; böylece Close() asla bekleyen I/O'ya takılmaz).
func (s *conptySession) withEvent(handle windows.Handle, fn func(ev windows.Handle) error) error {
	ev, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(ev)
	return fn(ev)
}

func (s *conptySession) Read(p []byte) (int, error) {
	s.mu.Lock()
	out := s.out
	s.mu.Unlock()
	if out == 0 {
		return 0, os.ErrClosed
	}
	var n uint32
	err := s.withEvent(out, func(ev windows.Handle) error {
		ov := &windows.Overlapped{HEvent: ev}
		return overlappedRW(func() error {
			return windows.ReadFile(out, p, &n, ov)
		}, out, ov, &n)
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *conptySession) Write(p []byte) (int, error) {
	s.mu.Lock()
	in := s.in
	s.mu.Unlock()
	if in == 0 {
		return 0, os.ErrClosed
	}
	var n uint32
	err := s.withEvent(in, func(ev windows.Handle) error {
		ov := &windows.Overlapped{HEvent: ev}
		return overlappedRW(func() error {
			return windows.WriteFile(in, p, &n, ov)
		}, in, ov, &n)
	})
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (s *conptySession) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	if conptyResize == nil {
		return fmt.Errorf("ConPTY bu Windows sürümünde kullanılamıyor")
	}
	s.mu.Lock()
	pc := s.pc
	s.mu.Unlock()
	if pc == 0 {
		return nil
	}
	r, _, err := conptyResize.Call(uintptr(pc), coordPacked(cols, rows))
	if r != 0 {
		return fmt.Errorf("ResizePseudoConsole: HRESULT=0x%X (%v)", r, err)
	}
	return nil
}

// teardown pipe ve ConPTY kaynaklarını bir kez serbest bırakır.
// (proc işaretçisi done() tarafından kapatılır; asıl iş parçacığı
// beklemesiyle çakışmasın.)
func (s *conptySession) teardown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tornDown {
		return
	}
	s.tornDown = true
	if s.pc != 0 {
		conptyClose.Call(uintptr(s.pc))
		s.pc = 0
	}
	if s.in != 0 {
		windows.CloseHandle(s.in)
		s.in = 0
	}
	if s.out != 0 {
		windows.CloseHandle(s.out)
		s.out = 0
	}
}

func (s *conptySession) Close() error {
	s.mu.Lock()
	if !s.tornDown && s.proc != 0 && !s.exited {
		windows.TerminateProcess(s.proc, 1)
	}
	s.mu.Unlock()
	s.teardown()
	return nil
}
