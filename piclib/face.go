package piclib

import (
	"database/sql"
	"sort"
)

type Face struct {
	FaceID int
	PicID  int
	X, Y   float64
	W, H   float64
	Model  string
	Vec    []float32
	Name   string
}

func (l *Lib) ReplaceFaces(picID int, model string, faces []Face) ([]Face, error) {
	tx, err := l.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	old, err := tx.Query("SELECT face_id FROM faces WHERE pic_id=? AND model=?", picID, model)
	if err != nil {
		return nil, err
	}
	var ids []int
	for old.Next() {
		var id int
		if err := old.Scan(&id); err != nil {
			old.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	old.Close()
	if err := old.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := tx.Exec("DELETE FROM face_person WHERE face_id=?", id); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec("DELETE FROM faces WHERE pic_id=? AND model=?", picID, model); err != nil {
		return nil, err
	}
	out := make([]Face, 0, len(faces))
	for _, f := range faces {
		vec := append([]float32(nil), f.Vec...)
		L2Normalize(vec)
		res, err := tx.Exec(
			"INSERT INTO faces (pic_id,x,y,w,h,model,dim,vec) VALUES (?,?,?,?,?,?,?,?)",
			picID, f.X, f.Y, f.W, f.H, model, len(vec), EncodeVec(vec),
		)
		if err != nil {
			return nil, err
		}
		fid, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		f.FaceID = int(fid)
		f.PicID = picID
		f.Model = model
		f.Vec = vec
		out = append(out, f)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (l *Lib) NeedsFaces(id int, model string) (bool, error) {
	var n int
	err := l.db.QueryRow("SELECT COUNT(*) FROM faces WHERE pic_id=? AND model=?", id, model).Scan(&n)
	if err != nil {
		return false, err
	}
	return n == 0, nil
}

func (l *Lib) GetFace(faceID int) (*Face, error) {
	f := &Face{FaceID: faceID}
	var raw []byte
	err := l.db.QueryRow(
		"SELECT pic_id,x,y,w,h,model,vec FROM faces WHERE face_id=?",
		faceID,
	).Scan(&f.PicID, &f.X, &f.Y, &f.W, &f.H, &f.Model, &raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.Vec = DecodeVec(raw)
	f.Name, err = l.faceName(faceID)
	return f, err
}

func (l *Lib) faceName(faceID int) (string, error) {
	var name string
	err := l.db.QueryRow(
		`SELECT p.name FROM face_person fp JOIN people p ON p.person_id=fp.person_id WHERE fp.face_id=?`,
		faceID,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return name, err
}

func (l *Lib) FacesByPic(picID int) ([]Face, error) {
	rows, err := l.db.Query(
		`SELECT f.face_id,f.pic_id,f.x,f.y,f.w,f.h,f.model,f.vec,COALESCE(p.name,'')
		 FROM faces f
		 LEFT JOIN face_person fp ON fp.face_id=f.face_id
		 LEFT JOIN people p ON p.person_id=fp.person_id
		 WHERE f.pic_id=?
		 ORDER BY f.face_id`,
		picID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFaces(rows)
}

func scanFaces(rows *sql.Rows) ([]Face, error) {
	var out []Face
	for rows.Next() {
		var f Face
		var raw []byte
		if err := rows.Scan(&f.FaceID, &f.PicID, &f.X, &f.Y, &f.W, &f.H, &f.Model, &raw, &f.Name); err != nil {
			return nil, err
		}
		f.Vec = DecodeVec(raw)
		out = append(out, f)
	}
	return out, rows.Err()
}

func (l *Lib) SimilarFaces(vec []float32, min float32) ([]Face, error) {
	L2Normalize(vec)
	rows, err := l.db.Query(
		`SELECT f.face_id,f.pic_id,f.x,f.y,f.w,f.h,f.model,f.vec,COALESCE(p.name,'')
		 FROM faces f
		 LEFT JOIN face_person fp ON fp.face_id=f.face_id
		 LEFT JOIN people p ON p.person_id=fp.person_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type scored struct {
		f Face
		s float32
	}
	var all []scored
	for rows.Next() {
		var f Face
		var raw []byte
		if err := rows.Scan(&f.FaceID, &f.PicID, &f.X, &f.Y, &f.W, &f.H, &f.Model, &raw, &f.Name); err != nil {
			return nil, err
		}
		f.Vec = DecodeVec(raw)
		s := Cosine(vec, f.Vec)
		if s >= min {
			all = append(all, scored{f, s})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].s > all[j].s })
	out := make([]Face, len(all))
	for i, s := range all {
		out[i] = s.f
	}
	return out, nil
}

func (l *Lib) FaceCount() (int, error) {
	n := 0
	err := l.db.QueryRow("SELECT COUNT(*) FROM faces").Scan(&n)
	return n, err
}
