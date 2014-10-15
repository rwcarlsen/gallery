// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

// TODO: mount groups of pics named nicely (maybe in nice dir structure) with
// softlinks

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"

	_ "github.com/mxk/go-sqlite/sqlite3"
	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

// rots holds mappings from exif orientation tag to degrees clockwise needed
var rots = map[int]int{
	1: 0,
	2: 0,
	3: 180,
	4: 180,
	5: 90,
	6: 90,
	7: 270,
	8: 270,
}

func DefaultPath() string {
	s := os.Getenv("PICLIB")
	if s != "" {
		return s
	}
	return filepath.Join(os.Getenv("HOME"), "piclib")
}

// db schema:
//
// * files
// 	- id INTEGER
// 	- sum BLOB
// 	- name TEXT
// 	- added INTEGER
// 	- taken INTEGER
// 	- orient INTEGER
// 	- thumb BLOB
// * meta
// 	- id INTEGER
// 	- time INTEGER
// 	- field TEXT
// 	- value TEXT

const Version = "0.1"
const Libname = "piclib.sqlite"
const NotesField = "Notes"
const thumbw = 1000
const thumbh = 0

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
	// check if file exists in libary
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

	// copy file into library path
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

	/////// add entry to meta database ////////
	// id, sum, name, added, taken, orient, thumb
	added := time.Now()
	taken := time.Time{}
	orient := 0

	// exif metadata
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
			orient = rots[int(v)]
		}
	}

	// make thumb
	f3, err := os.Open(pic)
	if err != nil {
		return nil, err
	}
	defer f3.Close()

	w, h := l.ThumbW, l.ThumbH
	if w == 0 && h == 0 {
		w, h = thumbw, thumbh
	}
	thumb, _ := MakeThumb(f3, w, h)

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

type Pic struct {
	id     int // for protection
	Id     int // for marshalling
	lib    *Lib
	Sum    []byte
	Name   string
	Added  time.Time
	Taken  time.Time
	Orient int
}

func (p *Pic) Filepath() string {
	return filepath.Join(p.lib.Path, diskname(p.Name, p.Sum))
}

func (p *Pic) Ext() string { return strings.ToLower(filepath.Ext(p.Name)) }

func (p *Pic) Open() (io.ReadCloser, error) {
	return os.Open(p.Filepath())
}

func (p *Pic) GetMeta(field string) (string, error) {
	s := "SELECT value FROM meta WHERE id=? ORDER BY time DESC LIMIT 1;"
	val := ""
	err := p.lib.db.QueryRow(s, p.id).Scan(&val)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	return val, nil
}

func (p *Pic) SetMeta(field, val string) error {
	s := "INSERT INTO meta (id,time,field,value) VALUES (?,?,?,?);"
	_, err := p.lib.db.Exec(s, p.id, time.Now(), field, val)
	return err
}

func (p *Pic) SetNotes(val string) error { return p.SetMeta(NotesField, val) }
func (p *Pic) GetNotes() (string, error) { return p.GetMeta(NotesField) }

func (p *Pic) Validate() error {
	rc, err := p.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	sum, err := Sha256(rc)
	if err != nil {
		return err
	}

	if !bytes.Equal(sum, p.Sum) {
		return BadSumErr(*p)
	}
	return nil
}

func (p *Pic) Thumb() ([]byte, error) {
	s := "SELECT thumb FROM files WHERE id=?"
	data := []byte{}
	err := p.lib.db.QueryRow(s, p.id).Scan(&data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type BadSumErr Pic

func (e BadSumErr) Error() string {
	p := Pic(e)
	return fmt.Sprintf("file '%v' (pic '%v') failed checksum validation", p.Filepath(), p.Name)
}

func IsBadSum(err error) bool {
	_, ok := err.(BadSumErr)
	return ok
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

func Sha256(r io.Reader) (sum []byte, err error) {
	h := sha256.New()
	_, err = io.Copy(h, r)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func MakeThumb(r io.Reader, w, h int) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	m := resize.Resize(uint(w), uint(h), img, resize.Bicubic)

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, m, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
