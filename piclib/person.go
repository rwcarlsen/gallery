package piclib

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

type Person struct {
	ID   int
	Name string
}

func (l *Lib) EnsurePerson(name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("person name required")
	}
	var id int
	err := l.db.QueryRow("SELECT person_id FROM people WHERE name=?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := l.db.Exec("INSERT INTO people (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	fid, err := res.LastInsertId()
	return int(fid), err
}

func (l *Lib) AssignPerson(faceID int, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		_, err := l.db.Exec("DELETE FROM face_person WHERE face_id=?", faceID)
		return err
	}
	pid, err := l.EnsurePerson(name)
	if err != nil {
		return err
	}
	_, err = l.db.Exec(
		`INSERT INTO face_person (face_id, person_id) VALUES (?,?)
		 ON CONFLICT(face_id) DO UPDATE SET person_id=excluded.person_id`,
		faceID, pid,
	)
	return err
}

func (l *Lib) ListPeople() ([]Person, error) {
	rows, err := l.db.Query("SELECT person_id, name FROM people ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (l *Lib) DeletePerson(name string) error {
	var id int
	err := l.db.QueryRow("SELECT person_id FROM people WHERE name=?", name).Scan(&id)
	if err == sql.ErrNoRows {
		return fmt.Errorf("no such person %q", name)
	}
	if err != nil {
		return err
	}
	if _, err := l.db.Exec("DELETE FROM face_person WHERE person_id=?", id); err != nil {
		return err
	}
	_, err = l.db.Exec("DELETE FROM people WHERE person_id=?", id)
	return err
}

func (l *Lib) PicsByPerson(name string) ([]int, error) {
	rows, err := l.db.Query(
		`SELECT DISTINCT f.pic_id
		 FROM faces f
		 JOIN face_person fp ON fp.face_id=f.face_id
		 JOIN people p ON p.person_id=fp.person_id
		 WHERE p.name=?
		 ORDER BY f.pic_id`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (l *Lib) FacesByPerson(name string) ([]Face, error) {
	rows, err := l.db.Query(
		`SELECT f.face_id,f.pic_id,f.x,f.y,f.w,f.h,f.model,f.vec,p.name
		 FROM faces f
		 JOIN face_person fp ON fp.face_id=f.face_id
		 JOIN people p ON p.person_id=fp.person_id
		 WHERE p.name=?
		 ORDER BY f.face_id`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFaces(rows)
}

func (l *Lib) PersonCentroid(name string) ([]float32, error) {
	faces, err := l.FacesByPerson(name)
	if err != nil {
		return nil, err
	}
	if len(faces) == 0 {
		return nil, fmt.Errorf("no faces named %q", name)
	}
	dim := len(faces[0].Vec)
	sum := make([]float32, dim)
	for _, f := range faces {
		for i := 0; i < dim && i < len(f.Vec); i++ {
			sum[i] += f.Vec[i]
		}
	}
	L2Normalize(sum)
	return sum, nil
}

func (l *Lib) unnamedFaces() ([]Face, error) {
	rows, err := l.db.Query(
		`SELECT f.face_id,f.pic_id,f.x,f.y,f.w,f.h,f.model,f.vec,''
		 FROM faces f
		 LEFT JOIN face_person fp ON fp.face_id=f.face_id
		 WHERE fp.face_id IS NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFaces(rows)
}

func (l *Lib) PropagatePerson(name string, min float32) (int, error) {
	cent, err := l.PersonCentroid(name)
	if err != nil {
		return 0, err
	}
	unnamed, err := l.unnamedFaces()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range unnamed {
		if Cosine(cent, f.Vec) >= min {
			if err := l.AssignPerson(f.FaceID, name); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func (l *Lib) AutoAssignPic(picID int, min float32) (int, error) {
	people, err := l.ListPeople()
	if err != nil || len(people) == 0 {
		return 0, err
	}
	type ref struct {
		name string
		vec  []float32
	}
	var refs []ref
	for _, p := range people {
		v, err := l.PersonCentroid(p.Name)
		if err != nil {
			continue
		}
		refs = append(refs, ref{p.Name, v})
	}
	faces, err := l.FacesByPic(picID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range faces {
		if f.Name != "" {
			continue
		}
		bestName := ""
		best := min
		for _, r := range refs {
			s := Cosine(r.vec, f.Vec)
			if s >= best {
				best = s
				bestName = r.name
			}
		}
		if bestName != "" {
			if err := l.AssignPerson(f.FaceID, bestName); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

func IntersectIDs(pics []*Pic, ids []int) []*Pic {
	want := make(map[int]int, len(ids))
	for i, id := range ids {
		want[id] = i
	}
	out := make([]*Pic, 0, len(ids))
	for _, p := range pics {
		if _, ok := want[p.Id]; ok {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return want[out[i].Id] < want[out[j].Id] })
	return out
}

func OrderByIDs(pics []*Pic, ids []int) []*Pic {
	byID := make(map[int]*Pic, len(pics))
	for _, p := range pics {
		byID[p.Id] = p
	}
	out := make([]*Pic, 0, len(ids))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, p)
		}
	}
	return out
}
