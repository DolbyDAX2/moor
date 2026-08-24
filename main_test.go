package main

import (
	"reflect"
	"testing"
)

func TestParseSyncArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		url  string
		rest []string
	}{
		{
			name: "flagsiz URL",
			args: []string{"https://ornek.com"},
			url:  "https://ornek.com",
			rest: []string{},
		},
		{
			name: "URL + flagler",
			args: []string{"https://ornek.com", "--user", "adi", "--pass", "sifre12345"},
			url:  "https://ornek.com",
			rest: []string{"--user", "adi", "--pass", "sifre12345"},
		},
		{
			name: "URL olmadan flaglar (degerler URL sanilmamali)",
			args: []string{"--user", "dbg1", "--pass", "Test-123456"},
			url:  "",
			rest: []string{"--user", "dbg1", "--pass", "Test-123456"},
		},
		{
			name: "esitli flagler",
			args: []string{"--user=adi", "--pass=sifre12345"},
			url:  "",
			rest: []string{"--user=adi", "--pass=sifre12345"},
		},
		{
			name: "karmasik sirali",
			args: []string{"--pass", "sifre12345", "--user", "adi", "https://ornek.com"},
			url:  "https://ornek.com",
			rest: []string{"--pass", "sifre12345", "--user", "adi"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			url, rest := parseSyncArgs(c.args)
			if url != c.url {
				t.Errorf("url = %q, beklenen %q", url, c.url)
			}
			if !reflect.DeepEqual(rest, c.rest) {
				t.Errorf("rest = %#v, beklenen %#v", rest, c.rest)
			}
		})
	}
}
