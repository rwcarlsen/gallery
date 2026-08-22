package piclib

import (
	"database/sql"
	"sort"
)

func (l *Lib) UpsertClip(id int, model string, vec []float32) error {
	L2Normalize(vec)
	_, err := l.db.Exec(
		`INSERT INTO clip (id, model, dim, vec) VALUES (?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET model=excluded.model, dim=excluded.dim, vec=excluded.vec`,
		id, model, len(vec), EncodeVec(vec),
	)
	return err
}

func (l *Lib) ClipVec(id int) ([]float32, string, error) {
	var model string
	var raw []byte
	err := l.db.QueryRow("SELECT model, vec FROM clip WHERE id=?", id).Scan(&model, &raw)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	return DecodeVec(raw), model, nil
}

func (l *Lib) NeedsClip(id int, model string) (bool, error) {
	var m string
	err := l.db.QueryRow("SELECT model FROM clip WHERE id=?", id).Scan(&m)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return m != model, nil
}

func (l *Lib) ClipCount() (int, error) {
	n := 0
	err := l.db.QueryRow("SELECT COUNT(*) FROM clip").Scan(&n)
	return n, err
}

type ScoredID struct {
	ID    int
	Score float32
}

func (l *Lib) SimilarClip(vec []float32, min float32) ([]ScoredID, error) {
	L2Normalize(vec)
	rows, err := l.db.Query("SELECT id, vec FROM clip")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ScoredID
	for rows.Next() {
		var id int
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		s := Cosine(vec, DecodeVec(raw))
		if s >= min {
			out = append(out, ScoredID{ID: id, Score: s})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}
