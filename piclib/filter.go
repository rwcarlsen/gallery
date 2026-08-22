package piclib

import (
	"regexp"
)

func (l *Lib) NotesMap() (map[int]string, error) {
	rows, err := l.db.Query("SELECT id, value FROM meta WHERE field=? ORDER BY time ASC", NotesField)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := map[int]string{}
	for rows.Next() {
		var id int
		var val string
		if err := rows.Scan(&id, &val); err != nil {
			return nil, err
		}
		notes[id] = val
	}
	return notes, rows.Err()
}

func (l *Lib) FilterPics(pics []*Pic, noteRE, nameRE string) ([]*Pic, error) {
	var nre, mre *regexp.Regexp
	var err error
	if noteRE != "" {
		nre, err = regexp.Compile(noteRE)
		if err != nil {
			return nil, err
		}
	}
	if nameRE != "" {
		mre, err = regexp.Compile(nameRE)
		if err != nil {
			return nil, err
		}
	}

	var notes map[int]string
	if nre != nil {
		notes, err = l.NotesMap()
		if err != nil {
			return nil, err
		}
	}

	out := make([]*Pic, 0, len(pics))
	for _, p := range pics {
		if mre != nil && !mre.MatchString(p.Name) {
			continue
		}
		if nre != nil && !nre.MatchString(notes[p.Id]) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func (l *Lib) VisiblePics(pics []*Pic, u *User) ([]*Pic, error) {
	if u == nil {
		return pics, nil
	}
	var re *regexp.Regexp
	if u.NotesRE != "" {
		var err error
		re, err = regexp.Compile(u.NotesRE)
		if err != nil {
			return nil, err
		}
	}
	notes, err := l.NotesMap()
	if err != nil {
		return nil, err
	}
	out := make([]*Pic, 0, len(pics))
	for _, p := range pics {
		if u.visible(notes[p.Id], re) {
			out = append(out, p)
		}
	}
	return out, nil
}
