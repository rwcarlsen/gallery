package piclib

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mxk/go-sqlite/sqlite3"
	"github.com/rwcarlsen/goexif/exif"
)

const (
	Version    = "0.1"
	Libname    = "piclib.sqlite"
	NotesField = "Notes"
)

const thumbw, thumbh = 1000, 0

type Lib struct {
	Path           string
	db             *sql.DB
	ThumbW, ThumbH int
}

func Open(path string) (*Lib, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	dbpath := filepath.Join(path, Libname)
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY,sum BLOB,name TEXT,added INTEGER,taken INTEGER,orient INTEGER,thumb BLOB);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS meta (id INTEGER,time INTEGER,field TEXT,value TEXT);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS files_taken ON files (taken);")
	if err != nil {
		return nil, err
	}
	return &Lib{Path: path, db: db}, nil
}

func (l *Lib) Open(id int) (*Pic, error) {
	s := "SELECT id,sum,name,added,taken,orient FROM files WHERE id=?"
	p := &Pic{lib: l}
	var taken, added int64
	err := l.db.QueryRow(s, id).Scan(&p.id, &p.Sum, &p.Name, &added, &taken, &p.Orient)
	if err != nil {
		return nil, err
	}

	p.Taken = time.Unix(taken, 0)
	p.Added = time.Unix(added, 0)
	p.Id = p.id
	return p, nil
}

func (l *Lib) ListTime(start, end time.Time) (pics []*Pic, err error) {
	s := "SELECT id,sum,name,added,taken,orient FROM files"
	s += " WHERE taken >= ? AND taken <= ? ORDER BY taken DESC;"
	rows, err := l.db.Query(s, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var added, taken int64
	for rows.Next() {
		p := &Pic{lib: l}
		err := rows.Scan(&p.id, &p.Sum, &p.Name, &added, &taken, &p.Orient)
		if err != nil {
			return nil, err
		}
		p.Id = p.id
		p.Taken = time.Unix(taken, 0)
		p.Added = time.Unix(added, 0)
		pics = append(pics, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pics, nil
}

func (l *Lib) List(limit, offset int) (pics []*Pic, err error) {
	s := "SELECT id,sum,name,added,taken,orient FROM files ORDER BY taken DESC,added DESC"
	var rows *sql.Rows
	if limit > 0 {
		s += " LIMIT ? OFFSET ?"
		rows, err = l.db.Query(s, limit, offset)
	} else {
		rows, err = l.db.Query(s)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var added, taken int64
	for rows.Next() {
		p := &Pic{lib: l}
		err := rows.Scan(&p.id, &p.Sum, &p.Name, &added, &taken, &p.Orient)
		if err != nil {
			return nil, err
		}
		p.Id = p.id
		p.Taken = time.Unix(taken, 0)
		p.Added = time.Unix(added, 0)
		pics = append(pics, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pics, nil
}

func diskname(name string, sum []byte) string {
	return fmt.Sprintf("%x%s", sum, filepath.Ext(name))
}

func (l *Lib) stdpath(name string, r io.Reader) (path string, sum []byte, err error) {
	sum, err = Sha256(r)
	if err != nil {
		return "", nil, err
	}
	fname := diskname(name, sum)
	return filepath.Join(l.Path, fname), sum, nil
}

func (l *Lib) Exists(sum []byte) (exists bool, err error) {
	name := ""
	err = l.db.QueryRow("SELECT name FROM files WHERE sum=?", sum).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// Add copies the picture into and adds it to the current library
func (l *Lib) AddFile(pic string) (p *Pic, err error) {
	// check if file exists
	f, err := os.Open(pic)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	newname, sum, err := l.stdpath(pic, f)
	if err != nil {
		return nil, err
	}

	if exists, err := l.Exists(sum); err == nil && exists {
		return nil, DupErr{pic}
	} else if err != nil {
		return nil, err
	}

	// copy file into library
	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	dst, err := os.Create(newname)
	if err != nil {
		return nil, err
	}
	defer dst.Close()

	_, err = io.Copy(dst, f)
	if err != nil {
		return nil, err
	}
	err = dst.Chmod(0444)
	if err != nil {
		return nil, err
	}

	// get meta data and make thumb
	added := time.Now()
	taken := time.Time{}
	orient := 0

	_, err = f.Seek(0, os.SEEK_SET)
	if err != nil {
		return nil, err
	}

	x, err := exif.Decode(f)
	if err == nil {
		tm, err := x.DateTime()
		if err == nil {
			taken = tm
		}

		tag, err := x.Get(exif.Orientation)
		if err == nil {
			v, _ := tag.Int(0)
			orient = int(v)
		}
	}

	f3, err := os.Open(pic)
	if err != nil {
		return nil, err
	}
	defer f3.Close()

	w, h := l.ThumbW, l.ThumbH
	if w == 0 && h == 0 {
		w, h = thumbw, thumbh
	}
	thumb, _ := MakeThumb(f3, w, h, orient)

	// store meta data in db and return new Pic
	sql := "INSERT INTO files (sum, name, added, taken, orient, thumb) VALUES (?,?,?,?,?,?);"
	_, err = l.db.Exec(sql, sum, filepath.Base(pic), added, taken, orient, thumb)
	if err != nil {
		return nil, err
	}

	var id int
	err = l.db.QueryRow("SELECT id FROM files WHERE sum=?;", sum).Scan(&id)
	if err != nil {
		return nil, err
	}
	return l.Open(id)
}

type DupErr struct {
	pic string
}

func (s DupErr) Error() string {
	return fmt.Sprintf("%v already exists in the library", s.pic)
}

func IsDup(err error) bool {
	_, ok := err.(DupErr)
	return ok
}
