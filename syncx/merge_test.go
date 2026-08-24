package syncx

import (
	"testing"
	"time"

	"moor/model"
)

func ms(id, name string, updated time.Time) *model.Server {
	return &model.Server{ID: id, Name: name, UpdatedAt: updated}
}

func TestMergeServersUnion(t *testing.T) {
	t0 := time.Now().Add(-time.Hour)
	t1 := t0.Add(time.Minute)
	t2 := t0.Add(2 * time.Minute)

	local := []*model.Server{
		ms("a", "arm", t2), // her iki tarafta da var, yerel yenisi
		ms("b", "vps", t1), // her iki tarafta da var, uzak yenisi
		ms("c", "local-only", t0),
	}
	remote := []*model.Server{
		ms("b", "vps", t2),     // uzak yenisi -> uzak kazanır
		ms("a", "arm-old", t1), // yerel yenisi -> yerel kazanır
		ms("d", "remote-only", t0),
	}

	out := MergeServers(local, remote)
	if len(out) != 4 {
		t.Fatalf("4 kayıt bekleniyor, %d geldi", len(out))
	}
	byID := map[string]*model.Server{}
	for _, s := range out {
		byID[s.ID] = s
	}
	if byID["a"].Name != "arm" || !byID["a"].UpdatedAt.Equal(t2) {
		t.Error("a: yerel (yenisi) kazanmalı")
	}
	if byID["b"].UpdatedAt != t2 {
		t.Error("b: uzak (yenisi) kazanmalı")
	}
	if byID["c"] == nil {
		t.Error("c: yerel-only korunmalı")
	}
	if byID["d"] == nil {
		t.Error("d: remote-only eklenmeli")
	}
	// kararlı sıra: önce yerel (a,b,c), sonra uzakta olanlar (d)
	if out[0].ID != "a" || out[1].ID != "b" || out[2].ID != "c" || out[3].ID != "d" {
		ids := []string{out[0].ID, out[1].ID, out[2].ID, out[3].ID}
		t.Errorf("beklenmeyen sıra: %v", ids)
	}
}

func TestMergeServersEmpty(t *testing.T) {
	out := MergeServers(nil, nil)
	if len(out) != 0 {
		t.Fatal("iki boş liste boş olmalı")
	}
	local := []*model.Server{ms("a", "a", time.Now())}
	out = MergeServers(local, nil)
	if len(out) != 1 || out[0].ID != "a" {
		t.Fatal("boş uzak + yerel -> yerel korunmalı")
	}
	out = MergeServers(nil, local)
	if len(out) != 1 || out[0].ID != "a" {
		t.Fatal("boş yerel + uzak -> uzak korunmalı")
	}
}

func TestMergeServersTieGoesLocal(t *testing.T) {
	ts := time.Now()
	l := []*model.Server{ms("a", "local", ts)}
	r := []*model.Server{ms("a", "remote", ts)} // aynı UpdatedAt
	out := MergeServers(l, r)
	if len(out) != 1 || out[0].Name != "local" {
		t.Fatal("beraberlikte yerel kazanmalı")
	}
}
