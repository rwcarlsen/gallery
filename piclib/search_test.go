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

func TestAssignPersonEmptyUnassigns(t *testing.T) {
	l := testLib(t)
	p := addPic(t, l, "a.jpg", 1)
	faces, err := l.ReplaceFaces(p.Id, "f", []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Vec: []float32{1, 0}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AssignPerson(faces[0].FaceID, "ada"); err != nil {
		t.Fatal(err)
	}
	if err := l.AssignPerson(faces[0].FaceID, "  "); err != nil {
		t.Fatal(err)
	}
	got, err := l.FacesByPic(p.Id)
	if err != nil || len(got) != 1 || got[0].Name != "" {
		t.Fatalf("%+v err=%v", got, err)
	}
	people, err := l.ListPeople()
	if err != nil || len(people) != 1 || people[0].Name != "ada" {
		t.Fatalf("person stays: %v %v", people, err)
	}
}

func TestOrderByIDs(t *testing.T) {
	pics := []*Pic{{Id: 3}, {Id: 1}, {Id: 2}}
	got := OrderByIDs(pics, []int{2, 3, 9})
	if len(got) != 2 || got[0].Id != 2 || got[1].Id != 3 {
		t.Fatalf("%v", ids(got))
	}
}

func TestReindexWipesNames(t *testing.T) {
	l := testLib(t)
	p1 := addPic(t, l, "a.jpg", 1)
	p2 := addPic(t, l, "b.jpg", 2)
	f1, err := l.ReplaceFaces(p1.Id, "buffalo-s", []Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Vec: []float32{1, 0}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AssignPerson(f1[0].FaceID, "ada"); err != nil {
		t.Fatal(err)
	}
	_, err = l.ReplaceFaces(p2.Id, "buffalo-s", []Face{{X: 0.2, Y: 0.2, W: 0.2, H: 0.2, Vec: []float32{0.99, 0.01}}})
	if err != nil {
		t.Fatal(err)
	}
	n, err := l.AutoAssignPic(p2.Id, 0.9)
	if err != nil || n != 1 {
		t.Fatalf("auto n=%d err=%v", n, err)
	}
	need, err := l.NeedsFaces(p2.Id, "buffalo-s")
	if err != nil || need {
		t.Fatalf("need same=%v err=%v", need, err)
	}
	need, err = l.NeedsFaces(p2.Id, "other")
	if err != nil || !need {
		t.Fatalf("need other=%v (zero rows, not model mismatch)", need)
	}
	_, err = l.ReplaceFaces(p2.Id, "buffalo-s", []Face{{X: 0.2, Y: 0.2, W: 0.2, H: 0.2, Vec: []float32{0.99, 0.01}}})
	if err != nil {
		t.Fatal(err)
	}
	faces, err := l.FacesByPic(p2.Id)
	if err != nil || len(faces) != 1 || faces[0].Name != "" {
		t.Fatalf("wiped names: %+v err=%v", faces, err)
	}
	n, err = l.AutoAssignPic(p2.Id, 0.9)
	if err != nil || n != 1 {
		t.Fatalf("restore n=%d err=%v", n, err)
	}
	if err := l.DeletePerson("ada"); err != nil {
		t.Fatal(err)
	}
	ids, err := l.PicsByPerson("ada")
	if err != nil || len(ids) != 0 {
		t.Fatalf("after rm %v err=%v", ids, err)
	}
	faces, err = l.FacesByPic(p1.Id)
	if err != nil || len(faces) != 1 || faces[0].Name != "" {
		t.Fatalf("faces stay unnamed: %+v err=%v", faces, err)
	}
}

func TestSimilarFacesIntersect(t *testing.T) {
	l := testLib(t)
	p1 := addPic(t, l, "a.jpg", 1)
	p2 := addPic(t, l, "b.jpg", 2)
	p3 := addPic(t, l, "c.jpg", 3)
	a, err := l.ReplaceFaces(p1.Id, "f", []Face{{Vec: []float32{1, 0}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.ReplaceFaces(p2.Id, "f", []Face{{Vec: []float32{0.95, 0.05}}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.ReplaceFaces(p3.Id, "f", []Face{{Vec: []float32{0, 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.AssignPerson(a[0].FaceID, "ada"); err != nil {
		t.Fatal(err)
	}
	hits, err := l.SimilarFaces([]float32{1, 0}, 0.5)
	if err != nil || len(hits) != 2 || hits[0].PicID != p1.Id || hits[1].PicID != p2.Id {
		t.Fatalf("%+v err=%v", hits, err)
	}
	all := []*Pic{p3, p1, p2}
	got := IntersectIDs(all, []int{p1.Id})
	if len(got) != 1 || got[0].Id != p1.Id {
		t.Fatalf("intersect %v", ids(got))
	}
}
