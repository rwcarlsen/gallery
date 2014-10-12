// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"time"

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

func DefaultLibpath() string {
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
// 	- modified INTEGER
// 	- taken INTEGER
// 	- orient INTEGER
// 	- thumb BLOB
// * meta
// 	- id INTEGER
// 	- time INTEGER
// 	- field TEXT
// 	- value TEXT

const Version = "0.1"
const nameTimeFmt = "2006-01-02-15-04-05"
const libname = "meta.sqlite"
const thumbw = 1000
const thumbh = 0

type Lib struct {
	Path           string
	db             *sql.DB
	ThumbW, ThumbH int
}

func Open(path string) (*Lib, error) {
	dbpath := filepath.Join(path, libname)
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY,sum BLOB,name TEXT,added INTEGER,modified INTEGER,taken INTEGER,orient INTEGER,thumb BLOB);")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS meta (id INTEGER,time INTEGER,field TEXT,value TEXT);")
	if err != nil {
		return nil, err
	}
	return &Lib{Path: path, db: db}, nil
}

type Pic struct {
	id       int
	lib      *Lib
	Sum      []byte
	Name     string
	Added    time.Time
	Modified time.Time
	Taken    time.Time
	Orient   int
}

func (p *Pic) Filepath() string {
	return filepath.Join(p.lib.Path, diskname(p.Name, p.Sum))
}

func (p *Pic) Open() (io.ReadCloser, error) {
	return os.Open(p.Filepath())
}

func (p *Pic) Meta(field string) (string, error) {
	s := "SELECT value FROM meta WHERE id=? ORDER BY time DESC LIMIT 1;"
	val := ""
	err := p.lib.db.QueryRow(s, p.id).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

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

func (l *Lib) List(limit, offset int) (pics []*Pic, err error) {
	s := "SELECT id,sum,name,added,modified,taken,orient FROM files ORDER BY (taken,added)"
	var rows *sql.Rows
	if limit > 0 {
		s += "LIMIT ? OFFSET ?"
		rows, err = l.db.Query(s, limit, offset)
	} else {
		s += "OFFSET ?"
		rows, err = l.db.Query(s, offset)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		p := &Pic{}
		err := rows.Scan(&p.id, &p.Sum, &p.Name, &p.Added,
			&p.Modified, &p.Taken, &p.Orient)
		if err != nil {
			return nil, err
		}
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

func (l *Lib) Exists(sum []byte) (exists bool, name string, err error) {
	err = l.db.QueryRow("SELECT name FROM files WHERE sum=?", sum).Scan(&name)
	if err != nil {
		return false, "", err
	}
	return name != "", name, nil
}

// Add copies the picture into and adds it to the current library
func (l *Lib) Add(pic string) error {
	// make pic lib dir if it doesn't exist
	if err := os.MkdirAll(l.Path, 0755); err != nil {
		return err
	}

	// check if file exists in libary
	f, err := os.Open(pic)
	if err != nil {
		return err
	}
	newname, sum, err := l.stdpath(pic, f)
	f.Close()
	if err != nil {
		return err
	}

	if exists, name, err := l.Exists(sum); err == nil && exists {
		return DupErr{pic, name}
	} else if err != nil {
		return err
	}

	// copy file into library path
	f1, err := os.Open(pic)
	if err != nil {
		return err
	}
	defer f1.Close()

	f2, err := os.Create(newname)
	if err != nil {
		return err
	}
	defer f2.Close()

	_, err = io.Copy(f2, f1)
	if err != nil {
		return err
	}

	/////// add entry to meta database ////////
	// id, sum, name, added, modified, taken, orient, thumb
	added := time.Now()
	modified := time.Now()
	taken := time.Time{}
	orient := 0

	// exif metadata
	x, err := exif.Decode(f)
	if err == nil {
		t, err := x.DateTime()
		if err == nil {
			taken = t
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
		return err
	}
	defer f3.Close()

	w, h := l.ThumbW, l.ThumbH
	if w == 0 && h == 0 {
		w, h = thumbw, thumbh
	}
	thumb, _ := MakeThumb(f3, w, h)

	sql := "INSERT INTO files (sum, name, added, modified, taken, orient, thumb) VALUES (?,?,?,?,?,?,?);"
	_, err = l.db.Exec(sql, sum, filepath.Base(pic), added, modified, taken, orient, thumb)
	return err
}

type BadSumErr Pic

func (e BadSumErr) Error() string {
	p := Pic(e)
	return fmt.Sprintf("file '%v' (pic '%v') failed checksum validation", p.Filepath(), p.Sum)
}

func IsBadSum(err error) bool {
	_, ok := err.(BadSumErr)
	return ok
}

type DupErr struct {
	pic  string
	prev string
}

func (s DupErr) Error() string {
	return fmt.Sprintf("%v already exists as %v in the library", s.pic, s.prev)
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
