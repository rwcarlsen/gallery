package piclib

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAddFileDupAndValidate(t *testing.T) {
	l := testLib(t)
	src := filepath.Join(t.TempDir(), "shot.jpg")
	writeJPEG(t, src, 9)
	p, err := l.AddFile(src)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p.Filepath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0444 {
		t.Fatalf("perm %v", fi.Mode().Perm())
	}
	if _, err := l.AddFile(src); !IsDup(err) {
		t.Fatalf("dup: %v", err)
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p.Filepath(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Filepath(), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := p.Validate(); !IsBadSum(err) {
		t.Fatalf("want bad sum, got %v", err)
	}
}

func TestNotesLog(t *testing.T) {
	l := testLib(t)
	p := addPic(t, l, "n.jpg", 1)
	if err := p.SetNotes("a"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetNotes("a"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetNotes("b"); err != nil {
		t.Fatal(err)
	}
	got, err := p.GetNotes()
	if err != nil || got != "b" {
		t.Fatalf("notes=%q err=%v", got, err)
	}
	m, err := l.NotesMap()
	if err != nil || m[p.Id] != "b" {
		t.Fatalf("map=%v err=%v", m, err)
	}
	var n int
	if err := l.db.QueryRow("SELECT COUNT(*) FROM meta WHERE id=? AND field=?", p.id, NotesField).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows=%d want 2 (no-op skip)", n)
	}
}

func setTaken(t *testing.T, l *Lib, id int, taken time.Time) {
	t.Helper()
	if _, err := l.db.Exec("UPDATE files SET taken=? WHERE id=?", taken.Unix(), id); err != nil {
		t.Fatal(err)
	}
}

func TestListTimeAndClipOrder(t *testing.T) {
	l := testLib(t)
	old := addPic(t, l, "2019.jpg", 1)
	hit := addPic(t, l, "2020.jpg", 2)
	miss := addPic(t, l, "2020b.jpg", 3)
	setTaken(t, l, old.Id, time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC))
	setTaken(t, l, hit.Id, time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC))
	setTaken(t, l, miss.Id, time.Date(2020, 7, 1, 0, 0, 0, 0, time.UTC))
	if err := l.UpsertClip(old.Id, "m", []float32{1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := l.UpsertClip(hit.Id, "m", []float32{0.9, 0.1}); err != nil {
		t.Fatal(err)
	}
	if err := l.UpsertClip(miss.Id, "m", []float32{0, 1}); err != nil {
		t.Fatal(err)
	}

	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2020, 12, 31, 23, 59, 59, 0, time.UTC)
	pics, err := l.ListTime(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if len(pics) != 2 || pics[0].Id != miss.Id || pics[1].Id != hit.Id {
		t.Fatalf("date window %v", ids(pics))
	}
	scored, err := l.SimilarClip([]float32{1, 0}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	rank := make([]int, len(scored))
	for i, s := range scored {
		rank[i] = s.ID
	}
	got := OrderByIDs(pics, rank)
	if len(got) != 1 || got[0].Id != hit.Id {
		t.Fatalf("clip after date %v (old=%d still closer globally)", ids(got), old.Id)
	}
}
