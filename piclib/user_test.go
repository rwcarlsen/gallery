package piclib

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUserVisible(t *testing.T) {
	cases := []struct {
		notes     string
		re        string
		hideEmpty bool
		want      bool
	}{
		{"", "", false, true},
		{"", "", true, false},
		{"vacation", "", true, true},
		{"vacation", "vacation|kids", false, true},
		{"work", "vacation|kids", false, false},
		{"", "vacation|kids", false, false},
		{"", "vacation|kids", true, false},
		{"  kids trip", "kids", false, true},
		{"unrelated", "", false, true},
	}
	for _, c := range cases {
		u := &User{NotesRE: c.re, HideEmpty: c.hideEmpty}
		if got := u.Visible(c.notes); got != c.want {
			t.Errorf("Visible(%q) re=%q hide=%v got %v want %v", c.notes, c.re, c.hideEmpty, got, c.want)
		}
	}
}

func TestHashPassword(t *testing.T) {
	h, err := HashPassword("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "s3cret") {
		t.Fatal("good password rejected")
	}
	if CheckPassword(h, "wrong") {
		t.Fatal("bad password accepted")
	}
}

func TestValidNotesRE(t *testing.T) {
	if err := ValidNotesRE("vacation|kids"); err != nil {
		t.Fatal(err)
	}
	if err := ValidNotesRE("("); err == nil {
		t.Fatal("expected invalid regex")
	}
}

func testLib(t *testing.T) *Lib {
	t.Helper()
	dir := t.TempDir()
	l, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func writeJPEG(t *testing.T, path string, seed byte) {
	t.Helper()
	if err := os.WriteFile(path, []byte{0xff, 0xd8, 0xff, seed, 0, 1, 2, 3}, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGetMetaField(t *testing.T) {
	l := testLib(t)
	path := filepath.Join(l.Path, "a.jpg")
	writeJPEG(t, path, 1)
	p, err := l.AddFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SetNotes("hello"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetMeta("Other", "secret"); err != nil {
		t.Fatal(err)
	}
	got, err := p.GetNotes()
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("GetNotes=%q", got)
	}
	got, err = p.GetMeta("Other")
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret" {
		t.Fatalf("GetMeta(Other)=%q", got)
	}
}

func TestUserAuthAndFilter(t *testing.T) {
	l := testLib(t)
	hash, err := HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	n, err := l.UserCount()
	if err != nil || n != 0 {
		t.Fatalf("empty lib users=%d err=%v", n, err)
	}
	err = l.AddUser(&User{Name: "alice", PassHash: hash, NotesRE: "vac", HideEmpty: true})
	if err != nil {
		t.Fatal(err)
	}
	n, err = l.UserCount()
	if err != nil || n != 1 {
		t.Fatalf("after add users=%d err=%v", n, err)
	}
	u, err := l.AuthUser("alice", "pw")
	if err != nil || u == nil {
		t.Fatalf("auth: u=%v err=%v", u, err)
	}
	bad, err := l.AuthUser("alice", "no")
	if err != nil || bad != nil {
		t.Fatalf("bad auth: %v %v", bad, err)
	}

	a := filepath.Join(l.Path, "vac.jpg")
	b := filepath.Join(l.Path, "work.jpg")
	c := filepath.Join(l.Path, "empty.jpg")
	writeJPEG(t, a, 1)
	writeJPEG(t, b, 2)
	writeJPEG(t, c, 3)
	pa, err := l.AddFile(a)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := l.AddFile(b)
	if err != nil {
		t.Fatal(err)
	}
	pc, err := l.AddFile(c)
	if err != nil {
		t.Fatal(err)
	}
	if err := pa.SetNotes("vacation 2018"); err != nil {
		t.Fatal(err)
	}
	if err := pb.SetNotes("work trip"); err != nil {
		t.Fatal(err)
	}

	pics, err := l.List(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	vis, err := l.VisiblePics(pics, u)
	if err != nil {
		t.Fatal(err)
	}
	if len(vis) != 1 || vis[0].Id != pa.Id {
		t.Fatalf("visible=%v want only %d", ids(vis), pa.Id)
	}

	named, err := l.FilterPics(pics, "", `work`)
	if err != nil {
		t.Fatal(err)
	}
	if len(named) != 1 || named[0].Id != pb.Id {
		t.Fatalf("name filter=%v", ids(named))
	}
	noted, err := l.FilterPics(pics, "vacation", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(noted) != 1 || noted[0].Id != pa.Id {
		t.Fatalf("note filter=%v extra empty=%d", ids(noted), pc.Id)
	}
}

func ids(pics []*Pic) []int {
	out := make([]int, len(pics))
	for i, p := range pics {
		out[i] = p.Id
	}
	return out
}
