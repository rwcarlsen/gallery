package piclib

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rwcarlsen/goexif/exif"
)

const (
	Version    = "0.3"
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
	_, err = db.Exec("PRAGMA page_size = 8192;")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (name TEXT PRIMARY KEY,pass_hash TEXT NOT NULL,can_write INTEGER NOT NULL DEFAULT 0,notes_re TEXT NOT NULL DEFAULT '',hide_empty INTEGER NOT NULL DEFAULT 0);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS clip (id INTEGER PRIMARY KEY,model TEXT NOT NULL,dim INTEGER NOT NULL,vec BLOB NOT NULL);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS faces (face_id INTEGER PRIMARY KEY,pic_id INTEGER NOT NULL,x REAL NOT NULL,y REAL NOT NULL,w REAL NOT NULL,h REAL NOT NULL,model TEXT NOT NULL,dim INTEGER NOT NULL,vec BLOB NOT NULL);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS people (person_id INTEGER PRIMARY KEY,name TEXT UNIQUE NOT NULL);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS face_person (face_id INTEGER PRIMARY KEY,person_id INTEGER NOT NULL);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS faces_pic ON faces (pic_id);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS face_person_person ON face_person (person_id);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS files_taken ON files (taken,id,sum,name,added,orient);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS files_sum ON files (sum,id,name,added,taken,orient);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE INDEX IF NOT EXISTS files_id ON files (id,sum,name,added,taken,orient);")
	if err != nil {
		return nil, err
	}
	return &Lib{Path: path, db: db}, nil
}

func (l *Lib) ModelsDir() string {
	return filepath.Join(l.Path, "models")
}

func (l *Lib) PicsByIDs(ids []int) ([]*Pic, error) {
	out := make([]*Pic, 0, len(ids))
	for _, id := range ids {
		p, err := l.Open(id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (l *Lib) Close() error {
	if l.db == nil {
		return nil
	}
	return l.db.Close()
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
	rows, err := l.db.Query(s, start.Unix(), end.Unix())
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
	f, err := os.Open(pic)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sum, err := Sha256(f)
	if err != nil {
		return nil, err
	}
	if exists, err := l.Exists(sum); err != nil {
		return nil, err
	} else if exists {
		return nil, DupErr{pic}
	}

	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	newname := filepath.Join(l.Path, diskname(pic, sum))
	dst, err := os.Create(newname)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	if _, err = io.Copy(dst, f); err != nil {
		return nil, err
	}
	if err = dst.Chmod(0444); err != nil {
		return nil, err
	}
	if _, err = dst.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	added := time.Now()
	taken := time.Time{}
	orient := 0

	x, err := exif.Decode(dst)
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
	if _, err = dst.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	tw, th := l.ThumbW, l.ThumbH
	if tw == 0 && th == 0 {
		tw, th = thumbw, thumbh
	}
	thumb, _ := MakeThumb(dst, tw, th, orient)

	res, err := l.db.Exec("INSERT INTO files (sum, name, added, taken, orient, thumb) VALUES (?,?,?,?,?,?);", sum, filepath.Base(pic), added.Unix(), taken.Unix(), orient, thumb)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return l.Open(int(id))
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
