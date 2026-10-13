// moor: uzak sunucu kaydetme ve kontrol aracı. TUI, CLI ve GUI tek binary.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"

	"moor/gui"
	"moor/model"
	"moor/sshcfg"
	"moor/sshx"
	"moor/store"
	"moor/syncx"
)

const version = "0.1.2"

func main() {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		usage()
		return
	}

	st, err := loadStore()
	if err != nil {
		fatal(err)
	}

	// Argümansız: masaüstü (GUI) arayüzünü aç.
	if len(args) == 0 {
		if err := gui.Run(); err != nil {
			fatal(err)
		}
		return
	}

	switch args[0] {
	case "gui":
		if err := gui.Run(); err != nil {
			fatal(err)
		}
	case "version":
		fmt.Printf("moor %s\n", version)
	case "ls":
		cmdLS(st, args[1:])
	case "add":
		cmdAdd(st, args[1:])
	case "rm":
		cmdRM(st, args[1:])
	case "ssh":
		cmdSSH(st, args[1:])
	case "run":
		cmdRun(st, args[1:])
	case "scp":
		cmdSCP(st, args[1:])
	case "rsync":
		cmdRsync(st, args[1:])
	case "sftp":
		cmdSFTP(st, args[1:])
	case "fwd":
		cmdFwd(st, args[1:])
	case "ping":
		cmdPing(st, args[1:])
	case "test":
		cmdTest(st, args[1:])
	case "import":
		cmdImport(st, args[1:])
	case "export":
		cmdExport(st)
	case "login":
		cmdLogin(st, args[1:])
	case "register":
		cmdRegister(st, args[1:])
	case "logout":
		cmdLogout()
	case "sync":
		cmdSync(st)
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen komut: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`moor — uzak sunucu kaydetme ve kontrol aracı

Kullanım:
  moor                     masaüstü (GUI) uygulamasını açar
  moor gui                 aynı (GUI'yi açar)
  moor ls [--json]         kayıtlı sunucuları listeler
  moor add                 sunucu ekler (flag'ler aşağıda)
  moor rm AD               sunucu siler
  moor ssh AD [ek arg...]  SSH oturumu açar
  moor run AD KOMUT...     uzaktan komut çalıştırır
  moor scp AD KAYNAK HEDEF dosya kopyalar (scp)
  moor rsync AD KAYNAK HEDEF rsync ile eşitleme
  moor sftp AD             sftp istemcisi açar
  moor fwd AD YPORT UZAK_HOST UZAK_PORT
                           yerel port yönlendirme (Ctrl+C ile durur)
  moor ping [AD|all]       TCP erişilebilirlik + gecikme ölçer
  moor test AD             SSH kimlik doğrulama testi yapar
  moor import [--config DOSYA]
                           ~/.ssh/config'ten sunucu içe aktarır
  moor export              tüm veriyi JSON olarak yazar (yedek)
  moor register [--user AD] [--pass ŞİFRE]
                           yeni senkron hesabı oluşturur, giriş yapar ve eşitler
  moor login [--user AD] [--pass ŞİFRE]
                           senkron hesabına giriş yapar ve buluttan eşitler
  (URL belirtilmezse varsayılan sunucu kullanılır:
   " + syncx.DefaultBaseURL + ")
  moor logout              senkron oturumunu kapatır
  moor sync                sunucu listesini bulutla eşitler
  moor version
Veri dosyası: Linux ~/.config/moor/servers.json, Windows %APPDATA%\moor\servers.json
  (MOOR_CONFIG ortam değişkeniyle değiştirilebilir)

Windows notları:
  - ssh/scp/sftp için sistemde OpenSSH gerekir (Windows 10/11'de varsayılandır).
  - rsync ayrı kurulum veya WSL gerektirir; Windows ile birlikte gelmez.
  - Şifreli sunucularda sshpass yoktur: etkileşimli şifre sorusu sorulur.

moor add flag'leri:
  --name AD            (zorunlu)
  --host HOST          (zorunlu) IP veya alan adı
  --port 22            varsayılan 22
  --user root          boşsa yerel kullanıcı kullanılır
  --key /yol/anahtar   özel anahtar yolu (~ desteklenir)
  --password PASS      şifre (opsiyonel; sshpass ile otomatik, yoksa etkileşimli)
  --tags "a,b"         etiketler
  --notes "..."        notlar
  --extra "-4"         ek ssh argümanları (ör. jump host: "-J user@jump")
`)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "hata:", err)
	os.Exit(1)
}

func loadStore() (*store.Store, error) {
	path := os.Getenv("MOOR_CONFIG")
	if path == "" {
		var err error
		path, err = store.DefaultPath()
		if err != nil {
			return nil, err
		}
	}
	return store.Load(path)
}

func findServer(st *store.Store, name string) *model.Server {
	sv, i := st.FindByName(name)
	if i < 0 {
		fatal(fmt.Errorf("sunucu bulunamadı: %s (moor ls ile listelenir)", name))
	}
	return sv
}

// requireBinary, komutun sisteme özgü binary'sinin (ssh, scp, rsync vb.)
// kurulu olduğunu doğrular; yoksa açıklayıcı bir hata verir (Windows'ta
// rsync gibi varsayılan olmayan araçlar için).
func requireBinary(bin, hint string) {
	if _, err := exec.LookPath(bin); err != nil {
		msg := fmt.Sprintf("%s bu sistemde bulunamadı", bin)
		if hint != "" {
			msg += " — " + hint
		}
		fatal(errors.New(msg))
	}
}

func rsyncHint() string {
	if runtime.GOOS == "windows" {
		return "Windows'ta rsync ayrı kurulum veya WSL gerektirir"
	}
	return "önce rsync kurun"
}

func sshHint() string {
	if runtime.GOOS == "windows" {
		return "OpenSSH istemcisi gerekli (Windows 10/11'de Ayarlar → Uygulamalar → İsteğe bağlı özellikler → OpenSSH Client)"
	}
	return "önce OpenSSH kurun"
}

// --- komutlar ---

func cmdLS(st *store.Store, args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON çıktısı")
	fs.Parse(args)
	servers := st.List()
	// Teşhis kolaylığı: dosyalar okunabiliyorsa içeriğin dolu halini göster
	// (senkronun ne göndereceğinin aynası).
	syncx.BackfillKeyContent(servers)
	if *asJSON {
		out, _ := json.MarshalIndent(servers, "", "  ")
		fmt.Println(string(out))
		return
	}
	if len(servers) == 0 {
		fmt.Println("kayıtlı sunucu yok (moor add veya TUI'de 'a' ile ekleyin)")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "AD\tHOST\tPORT\tKULLANICI\tKIMLIK\tETİKETLER")
	for _, s := range servers {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
			s.Name, s.Host, s.Port, s.EffectiveUser(), s.AuthLabel(), strings.Join(s.Tags, ","))
	}
	w.Flush()
}

func cmdAdd(st *store.Store, args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	name := fs.String("name", "", "sunucu adı")
	host := fs.String("host", "", "IP veya alan adı")
	port := fs.Int("port", 22, "port")
	user := fs.String("user", "", "kullanıcı (boş = yerel kullanıcı)")
	keyPath := fs.String("key", "", "özel anahtar yolu")
	password := fs.String("password", "", "şifre (opsiyonel)")
	tags := fs.String("tags", "", "etiketler, virgülle ayrılmış")
	notes := fs.String("notes", "", "notlar")
	extra := fs.String("extra", "", "ek ssh argümanları")
	fs.Parse(args)

	if *name == "" || *host == "" {
		fatal(errors.New("--name ve --host zorunludur"))
	}
	sv := &model.Server{
		Name:         strings.TrimSpace(*name),
		Host:         strings.TrimSpace(*host),
		Port:         *port,
		User:         strings.TrimSpace(*user),
		KeyPath:      strings.TrimSpace(*keyPath),
		Password:     *password,
		Notes:        strings.TrimSpace(*notes),
		ExtraSSHArgs: strings.TrimSpace(*extra),
	}
	for _, t := range strings.Split(*tags, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			sv.Tags = append(sv.Tags, t)
		}
	}
	now := time.Now()
	sv.CreatedAt = now
	sv.UpdatedAt = now
	sv.Normalize()
	if err := sv.Validate(); err != nil {
		fatal(err)
	}
	st.Add(sv)
	if err := st.Save(); err != nil {
		fatal(err)
	}
	fmt.Printf("kaydedildi: %s (%s)\n", sv.Name, sv.Target())
}

func cmdRM(st *store.Store, args []string) {
	if len(args) < 1 {
		fatal(errors.New("kullanım: moor rm AD"))
	}
	sv, i := st.FindByName(args[0])
	if i < 0 {
		fatal(fmt.Errorf("sunucu bulunamadı: %s", args[0]))
	}
	st.Remove(sv.ID)
	if err := st.Save(); err != nil {
		fatal(err)
	}
	fmt.Printf("silindi: %s\n", sv.Name)
}

func cmdSSH(st *store.Store, args []string) {
	if len(args) < 1 {
		fatal(errors.New("kullanım: moor ssh AD [ek ssh argümanları...]"))
	}
	sv := findServer(st, args[0])
	requireBinary("ssh", sshHint())
	argv := append(sshx.SSHFullCommand(sv), args[1:]...)
	if err := sshx.RunInteractive(argv...); err != nil {
		fatal(err)
	}
}

func cmdRun(st *store.Store, args []string) {
	if len(args) < 2 {
		fatal(errors.New("kullanım: moor run AD KOMUT..."))
	}
	sv := findServer(st, args[0])
	requireBinary("ssh", sshHint())
	cmdText := strings.Join(args[1:], " ")
	res := sshx.RunCaptured(60*time.Second, sshx.RemoteCommandFull(sv, cmdText)...)
	if res.Stdout != "" {
		fmt.Print(res.Stdout)
	}
	if res.Stderr != "" {
		fmt.Fprint(os.Stderr, res.Stderr)
	}
	if res.Err != nil {
		fatal(res.Err)
	}
}

func cmdSCP(st *store.Store, args []string) {
	if len(args) < 3 {
		fatal(errors.New("kullanım: moor scp AD KAYNAK HEDEF"))
	}
	sv := findServer(st, args[0])
	requireBinary("scp", sshHint())
	if err := sshx.RunInteractive(sshx.SCPFullCommand(sv, args[1], args[2])...); err != nil {
		fatal(err)
	}
}

func cmdRsync(st *store.Store, args []string) {
	if len(args) < 3 {
		fatal(errors.New("kullanım: moor rsync AD KAYNAK HEDEF"))
	}
	sv := findServer(st, args[0])
	requireBinary("rsync", rsyncHint())
	if err := sshx.RunInteractive(sshx.RsyncFullCommand(sv, args[1], args[2])...); err != nil {
		fatal(err)
	}
}

func cmdSFTP(st *store.Store, args []string) {
	if len(args) < 1 {
		fatal(errors.New("kullanım: moor sftp AD"))
	}
	sv := findServer(st, args[0])
	requireBinary("sftp", sshHint())
	if err := sshx.RunInteractive(sshx.SFTPFullCommand(sv)...); err != nil {
		fatal(err)
	}
}

func cmdFwd(st *store.Store, args []string) {
	if len(args) < 4 {
		fatal(errors.New("kullanım: moor fwd AD YEREL_PORT UZAK_HOST UZAK_PORT"))
	}
	sv := findServer(st, args[0])
	lp, err1 := strconv.Atoi(args[1])
	rp, err2 := strconv.Atoi(args[3])
	if err1 != nil || err2 != nil {
		fatal(errors.New("portlar sayı olmalı"))
	}
	requireBinary("ssh", sshHint())
	fmt.Fprintf(os.Stderr, "yönlendirme: localhost:%d → %s:%d (Ctrl+C ile durdur)\n", lp, args[2], rp)
	if err := sshx.RunInteractive(sshx.ForwardFullCommand(sv, lp, args[2], rp)...); err != nil {
		fatal(err)
	}
}

func cmdPing(st *store.Store, args []string) {
	targets := st.List()
	if len(args) > 0 && args[0] != "all" {
		sv, i := st.FindByName(args[0])
		if i < 0 {
			fatal(fmt.Errorf("sunucu bulunamadı: %s", args[0]))
		}
		targets = []*model.Server{sv}
	}
	for _, s := range targets {
		d, err := sshx.TCPCheck(s.Host, s.Port, 3*time.Second)
		switch {
		case err != nil:
			fmt.Printf("%-20s %s:%d  ERİŞİLEMEZ (%v)\n", s.Name, s.Host, s.Port, err)
		default:
			fmt.Printf("%-20s %s:%d  OK (%dms)\n", s.Name, s.Host, s.Port, d.Milliseconds())
		}
	}
}

func cmdTest(st *store.Store, args []string) {
	if len(args) < 1 {
		fatal(errors.New("kullanım: moor test AD"))
	}
	sv := findServer(st, args[0])
	requireBinary("ssh", sshHint())
	res := sshx.SSHAuthCheck(sv, 15*time.Second)
	if res.Err == nil {
		fmt.Printf("%s: SSH oturum açma başarılı (%.2fs)\n", sv.Name, res.Duration.Seconds())
		return
	}
	detail := strings.TrimSpace(res.Stderr)
	if detail == "" {
		detail = res.Err.Error()
	}
	fmt.Fprintf(os.Stderr, "%s: başarısız — %s\n", sv.Name, strings.SplitN(detail, "\n", 2)[0])
	os.Exit(1)
}

func cmdImport(st *store.Store, args []string) {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	cfgPath := fs.String("config", "", "ssh config yolu (varsayılan ~/.ssh/config)")
	fs.Parse(args)
	if *cfgPath == "" {
		home, _ := os.UserHomeDir()
		*cfgPath = filepath.Join(home, ".ssh", "config")
	}
	entries, err := sshcfg.ParseFile(*cfgPath)
	if err != nil {
		fatal(fmt.Errorf("içe aktarma: %w", err))
	}
	added, skipped := 0, 0
	var names []string
	for _, sv := range sshcfg.ToServers(entries) {
		if _, i := st.FindByName(sv.Name); i >= 0 {
			skipped++
			continue
		}
		st.Add(sv)
		added++
		names = append(names, sv.Name)
	}
	if added > 0 {
		if err := st.Save(); err != nil {
			fatal(err)
		}
	}
	fmt.Printf("içe aktarma: %d eklendi (%s), %d atlandı (zaten kayıtlı)\n", added, strings.Join(names, ", "), skipped)
}

func cmdExport(st *store.Store) {
	out, err := json.MarshalIndent(st.List(), "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(out))
}

// --- senkron komutları ---

func promptLine(label string) string {
	fmt.Fprint(os.Stderr, label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

// parseSyncArgs, argümanları URL (konumsal) ve flag'lere ayırır.
// Değer alan flag'ler (--user/--pass) değerleriyle birlikte korunur; böylece
// "--user dbg1" içindeki "dbg1" URL sanılmaz.
func parseSyncArgs(args []string) (url string, rest []string) {
	rest = make([]string, 0, len(args))
	valueFlags := map[string]bool{"user": true, "pass": true}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			rest = append(rest, a)
			// Değer alan flag mi (--user AD / --pass ŞİFRE)? Değeri de
			// birlikte consume et; yoksa değer URL sanılır.
			base := a
			if idx := strings.Index(a, "="); idx >= 0 {
				base = a[:idx]
			}
			if valueFlags[strings.TrimLeft(base, "-")] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				rest = append(rest, args[i])
			}
			continue
		}
		if url == "" {
			url = a // ilk konumsal argüman = URL
		} else {
			rest = append(rest, a)
		}
	}
	return url, rest
}

func cmdLogin(st *store.Store, args []string) {
	// Go'nun flag paketi ilk konumsal argümanda parse'ı durdurur;
	// URL'yi herhangibir yerde kabul etmek için önceden ayıklayıp kalanları
	// flag olarak parse ediyoruz.
	urlStr, flagArgs := parseSyncArgs(args)
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	user := fs.String("user", "", "kullanıcı adı")
	pass := fs.String("pass", "", "şifre")
	fs.Parse(flagArgs)

	creds, err := syncx.LoadCredentials()
	if err != nil {
		fatal(err)
	}
	if urlStr == "" {
		urlStr = creds.ServerURL
	}
	if urlStr == "" {
		urlStr = syncx.DefaultBaseURL // varsayılan sunucu
	}
	if *user == "" && creds.Username != "" {
		*user = creds.Username
	}
	if *user == "" {
		*user = promptLine("kullanıcı adı: ")
	}
	if *pass == "" {
		fmt.Fprint(os.Stderr, "şifre: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil && len(b) == 0 {
			fatal(fmt.Errorf("şifre okunamadı: %w", err))
		}
		*pass = strings.TrimSpace(string(b))
	}

	c, err := syncx.NewClient(urlStr)
	if err != nil {
		fatal(err)
	}
	if err := c.Login(*user, *pass); err != nil {
		fatal(fmt.Errorf("giriş başarısız: %w", err))
	}
	creds.ServerURL = strings.TrimRight(urlStr, "/")
	creds.Username = *user
	creds.Token = c.Token()
	if err := creds.Save(); err != nil {
		fatal(err)
	}
	fmt.Printf("giriş yapıldı: %s (%s)\n", *user, creds.ServerURL)
	// Girişten sonra hemen bir eşitleme turu: buluttaki kayıtlı sunucular
	// bu cihaza gelsin. Hata girişin kendisini bozmasın.
	if err := runSyncRound(st); err != nil {
		fmt.Fprintf(os.Stderr, "uyarı: otomatik eşitleme başarısız: %v (daha sonra 'moor sync' ile deneyin)\n", err)
	}
}

// cmdRegister yeni hesap oluşturur ve giriş yapar (cmdLogin ile aynı akış).
func cmdRegister(st *store.Store, args []string) {
	// Aynen login gibi argüman ayrıştır; tek fark Register çağrısı.
	urlStr, flagArgs := parseSyncArgs(args)
	fs := flag.NewFlagSet("register", flag.ExitOnError)
	user := fs.String("user", "", "kullanıcı adı")
	pass := fs.String("pass", "", "şifre")
	fs.Parse(flagArgs)

	creds, err := syncx.LoadCredentials()
	if err != nil {
		fatal(err)
	}
	if urlStr == "" {
		urlStr = creds.ServerURL
	}
	if urlStr == "" {
		urlStr = syncx.DefaultBaseURL // varsayılan sunucu
	}
	if *user == "" && creds.Username != "" {
		*user = creds.Username
	}
	if *user == "" {
		*user = promptLine("kullanıcı adı: ")
	}
	if *pass == "" {
		fmt.Fprint(os.Stderr, "şifre (en az 8 karakter): ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil && len(b) == 0 {
			fatal(fmt.Errorf("şifre okunamadı: %w", err))
		}
		*pass = strings.TrimSpace(string(b))
	}

	c, err := syncx.NewClient(urlStr)
	if err != nil {
		fatal(err)
	}
	if err := c.Register(*user, *pass); err != nil {
		fatal(fmt.Errorf("kayıt başarısız: %w", err))
	}
	creds.ServerURL = strings.TrimRight(urlStr, "/")
	creds.Username = *user
	creds.Token = c.Token()
	if err := creds.Save(); err != nil {
		fatal(err)
	}
	fmt.Printf("hesap oluşturuldu ve giriş yapıldı: %s (%s)\n", *user, creds.ServerURL)
	if err := runSyncRound(st); err != nil {
		fmt.Fprintf(os.Stderr, "uyarı: otomatik eşitleme başarısız: %v (daha sonra 'moor sync' ile deneyin)\n", err)
	}
}

func cmdLogout() {
	creds, err := syncx.LoadCredentials()
	if err != nil {
		fatal(err)
	}
	if !creds.LoggedIn() {
		fmt.Println("açık bir oturum yok")
		return
	}
	if c, err := syncx.NewClient(creds.ServerURL); err == nil {
		c.SetToken(creds.Token)
		_ = c.Logout()
	}
	creds.Token = ""
	creds.LastSyncVersion = 0
	if err := creds.Save(); err != nil {
		fatal(err)
	}
	fmt.Println("çıkış yapıldı (kimlik bilgisi silindi, adres/kullanıcı saklandı)")
}

func cmdSync(st *store.Store) {
	creds, err := syncx.LoadCredentials()
	if err != nil {
		fatal(err)
	}
	if !creds.LoggedIn() {
		fatal(errors.New("önce giriş yapın: moor login"))
	}
	if err := runSyncRound(st); err != nil {
		fatal(err)
	}
}

// runSyncRound tam bir senkronizasyon turu yapar: depodaki listeyle bulutu
// birleştirir, sonucu hem dosyaya hem kimlik bilgisine yazar.
func runSyncRound(st *store.Store) error {
	creds, err := syncx.LoadCredentials()
	if err != nil {
		return err
	}
	if !creds.LoggedIn() {
		return errors.New("açık bir oturum yok")
	}
	c, err := syncx.NewClient(creds.ServerURL)
	if err != nil {
		return err
	}
	c.SetToken(creds.Token)

	merged, sum, err := syncx.SyncStore(c, st.List(), creds.LastSyncVersion)
	if err != nil {
		return err
	}
	st.ReplaceAll(merged)
	if err := st.Save(); err != nil {
		return err
	}
	creds.LastSyncVersion = sum.Version
	creds.LastSyncAt = sum.At
	if err := creds.Save(); err != nil {
		return err
	}
	fmt.Printf("senkron tamam: sürüm %d — yerel-only %d, uzak-only %d, çift taraflı değişiklik %d\n",
		sum.Version, sum.LocalOnly, sum.RemoteOnly, sum.ChangedBoth)
	return nil
}
