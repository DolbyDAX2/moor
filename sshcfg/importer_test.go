package sshcfg

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `# yorum satırı
Host oracle-prod
    HostName 203.0.113.10
    Port 22
    User root
    IdentityFile ~/.ssh/oracle_prod.pem

Host staging
  hostname example.com
  port 2222
  user ubuntu

Host *
    ServerAliveInterval 60

Host wildcard-*
    User foo
`

func TestParseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("4 Host bloğu bekleniyordu, %d geldi", len(entries))
	}

	e := entries[0]
	if e.Pattern != "oracle-prod" || e.Host != "203.0.113.10" || e.Port != 22 || e.User != "root" || e.KeyPath != "~/.ssh/oracle_prod.pem" {
		t.Fatalf("oracle-prod yanlış çözüldü: %+v", e)
	}
	e2 := entries[1]
	if e2.Pattern != "staging" || e2.Host != "example.com" || e2.Port != 2222 || e2.User != "ubuntu" {
		t.Fatalf("staging yanlış çözüldü: %+v", e2)
	}

	servers := ToServers(entries)
	if len(servers) != 2 {
		t.Fatalf("ToServers 2 sunucu bekleniyordu, %d geldi", len(servers))
	}
	if servers[0].Name != "oracle-prod" || servers[0].Host != "203.0.113.10" || servers[0].KeyPath != "~/.ssh/oracle_prod.pem" {
		t.Fatalf("oracle-prod sunucu yanlış: %+v", servers[0])
	}
	if servers[1].Name != "staging" || servers[1].Host != "example.com" || servers[1].Port != 2222 {
		t.Fatalf("staging sunucu yanlış: %+v", servers[1])
	}
	if servers[0].User == "" || servers[1].User == "" {
		t.Fatal("kullanıcı varsayılanı dolmamış")
	}
}
