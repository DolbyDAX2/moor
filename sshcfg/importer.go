// Package sshcfg ~/.ssh/config dosyasını okuyup sunucuları içe aktarır.
package sshcfg

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"moor/model"
)

// Entry ssh config'den çözülmüş bir Host bloğudur.
type Entry struct {
	Pattern string // Host satırındaki ilk desen
	Host    string // HostName (boşsa Pattern kullanılır)
	Port    int
	User    string
	KeyPath string // IdentityFile
}

// ParseFile ssh config dosyasını ayrıştırır.
func ParseFile(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []Entry
	var cur *Entry
	flush := func() {
		if cur != nil && cur.Pattern != "" {
			entries = append(entries, *cur)
		}
		cur = nil
	}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var key, val string
		if i := strings.Index(line, "="); i > 0 { // "Key = value" biçimi
			key = line[:i]
			val = strings.TrimSpace(line[i+1:])
		} else if i := strings.IndexAny(line, " \t"); i > 0 {
			key = line[:i]
			val = strings.TrimSpace(line[i+1:])
		} else {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.Trim(val, "\"'")
		switch key {
		case "host":
			flush()
			fields := strings.Fields(val)
			cur = &Entry{Pattern: fields[0]}
		case "hostname":
			if cur != nil && cur.Host == "" {
				cur.Host = val
			}
		case "port":
			if cur != nil {
				if p, err := strconv.Atoi(val); err == nil {
					cur.Port = p
				}
			}
		case "user":
			if cur != nil && cur.User == "" {
				cur.User = val
			}
		case "identityfile":
			if cur != nil && cur.KeyPath == "" {
				cur.KeyPath = val
			}
		}
	}
	flush()
	return entries, sc.Err()
}

// ToServers girdileri model.Server listesine çevirir; wildcard desenleri atlanır.
func ToServers(entries []Entry) []*model.Server {
	var out []*model.Server
	for _, e := range entries {
		if e.Pattern == "" || strings.ContainsAny(e.Pattern, "*?") {
			continue
		}
		host := e.Host
		if host == "" {
			host = e.Pattern
		}
		user := e.User
		if user == "" {
			user = model.CurrentUser()
		}
		sv := &model.Server{
			Name:    e.Pattern,
			Host:    host,
			Port:    e.Port,
			User:    user,
			KeyPath: e.KeyPath,
			Tags:    []string{"ssh-config"},
		}
		sv.Normalize()
		out = append(out, sv)
	}
	return out
}
