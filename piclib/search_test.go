package piclib

import (
	"os"
	"path/filepath"
	"testing"
)

func addPic(t *testing.T, l *Lib, name string, seed byte) *Pic {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte{0xff, 0xd8, 0xff, seed, 1, 2, 3, 4}, 0644); err != nil {
		t.Fatal(err)
	}
	p, err := l.AddFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCosineAndVec(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	c := []float32{0, 1, 0}
	if Cosine(a, b) < 0.99 {
		t.Fatal(Cosine(a, b))
	}
	if Cosine(a, c) > 0.01 {
		t.Fatal(Cosine(a, c))
	}
	raw := EncodeVec(a)
	got := DecodeVec(raw)
	if len(got) != 3 || got[0] != 1 {
		t.Fatalf("%v", got)
	}
}

func TestSimilarClip(t *testing.T) {
	l := testLib(t)
	p1 := addPic(t, l, "a.jpg", 1)
	p2 := addPic(t, l, "b.jpg", 2)
	p3 := addPic(t, l, "c.jpg", 3)
	if err := l.UpsertClip(p1.Id, "m", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := l.UpsertClip(p2.Id, "m", []float32{0.9, 0.1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := l.UpsertClip(p3.Id, "m", []float32{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	got, err := l.SimilarClip([]float32{1, 0, 0}, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != p1.Id || got[1].ID != p2.Id {
		t.Fatalf("%+v", got)
	}
	need, err := l.NeedsClip(p1.Id, "m")
	if err != nil || need {
		t.Fatalf("need=%v err=%v", need, err)
	}
	need, err = l.NeedsClip(p1.Id, "other")
	if err != nil || !need {
		t.Fatalf("need other=%v", need)
	}
}

func TestPersonAssignPropagate(t *testing.T) {
	l := testLib(t)
	p1 := addPic(t, l, "a.jpg", 1)
	p2 := addPic(t, l, "b.jpg", 2)
	faces, err := l.ReplaceFaces(p1.Id, "f", []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Vec: []float32{1, 0}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.ReplaceFaces(p2.Id, "f", []Face{{X: 0.2, Y: 0.2, W: 0.2, H: 0.2, Vec: []float32{0.99, 0.01}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AssignPerson(faces[0].FaceID, "ada"); err != nil {
		t.Fatal(err)
	}
	n, err := l.PropagatePerson("ada", 0.9)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("propagated %d", n)
	}
	ids, err := l.PicsByPerson("ada")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("pics %v", ids)
	}
	people, err := l.ListPeople()
	if err != nil || len(people) != 1 || people[0].Name != "ada" {
		t.Fatalf("%v %v", people, err)
	}
}

func TestOrderByIDs(t *testing.T) {
	pics := []*Pic{{Id: 3}, {Id: 1}, {Id: 2}}
	got := OrderByIDs(pics, []int{2, 3, 9})
	if len(got) != 2 || got[0].Id != 2 || got[1].Id != 3 {
		t.Fatalf("%v", ids(got))
	}
}
