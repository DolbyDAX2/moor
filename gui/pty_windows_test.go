//go:build windows

package gui

import (
	"reflect"
	"testing"
	"unicode/utf16"
)

func TestEnvBlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  []string
		want string
	}{
		{"multiple", []string{"ALPHA=1", "TERM=xterm-256color", "BETA=2"}, "ALPHA=1\x00TERM=xterm-256color\x00BETA=2\x00\x00"},
		{"empty", nil, "\x00\x00"},
		{"unicode", []string{"NAME=Türkçe"}, "NAME=Türkçe\x00\x00"},
		{"invalid", []string{"BAD=\x00", "GOOD=1"}, "GOOD=1\x00\x00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := envBlock(tc.env)
			want := utf16.Encode([]rune(tc.want))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("envBlock = %v, want %v", got, want)
			}
		})
	}
}
