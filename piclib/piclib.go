// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	_ "image/gif"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	db, err := db.Open(dbpath)
	if err != nil {
		return nil, err
	}

	_, err := db.Exec("CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY,sum BLOB,name TEXT,added INTEGER,modified INTEGER,taken INTEGER,orient INTEGER,thumb BLOB);")
	if err != nil {
		return nil, err
	}
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS meta (id INTEGER,time INTEGER,field TEXT,value TEXT);")
	if err != nil {
		return nil, err
	}
	return &Lib{Path: path, db: db}, nil
}

func (l *Lib) List(n int, skipext ...string) (pics []Pic, err error) {
}

func (l *Lib) stdname(name string, r io.Reader) (name string, sum []byte, err error) {
	sum, err := Sha256(r)
	if err != nil {
		return "", nil, err
	}
	fname := fmt.Sprintf("%x%s", sum, filepath.Ext(name))
	return filepath.Join(l.Path, fname), sum, nil
}

func (l *Lib) Exists(sum []byte) (exists bool, name string, err error) {
	name := ""
	err := db.QueryRow("SELECT name FROM files WHERE sum=?", sum).Scan(&name)
	if err != nil {
		return false, "", err
	}
	return name != "", name, nil
}

// Add copies a picture to the current library
func (l *Lib) Add(pic string) error {
	// make pic lib dir if it doesn't exist
	if err := os.MkdirAll(l.Path, 0755); err != nil {
		return "", err
	}

	// check if file exists in libary
	f, err := os.Open(pic)
	if err != nil {
		return err
	}
	newname, sum, err := stdname(pic, f)
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

	_, err := io.Copy(f2, f1)
	if err != nil {
		return err
	}

	/////// add entry to meta database ////////
	// id, sum, name, added, modified, taken, orient, thumb
	added := time.Now()
	modified := time.Now()
	taken := int64(0)
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
			v, _ := tg.Int(0)
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
	_, err := l.db.Exec(sql, sum, filepath.Base(pic), added, modified, taken, orient, thumb)
	return err
}

func (l *Lib) Validate(pic string) error {
	_, meta, err := Notes(pic)
	if err != nil {
		return err
	} else if meta == nil || len(meta.Sha1) == 0 {
		return NoSumErr(pic)
	}

	sum, err := Checksum(pic)
	if err != nil {
		return err
	}

	if meta.Sha1 != fmt.Sprintf("%x", sum) {
		return fmt.Errorf("%v failed validation", pic)
	}

	return nil
}

func canonicalName(pic string) (string, error) {
	b := filepath.Base(pic)
	if i := strings.Index(b, "-sep-"); i != -1 {
		b = b[i+len("-sep-"):]
	}

	t := Taken(pic)

	tm := t.Format(nameTimeFmt)
	if t.IsZero() {
		sum, err := Checksum(pic)
		if err != nil {
			return "", err
		}
		tm = fmt.Sprintf("%x-", sum)
		return NoDate + tm + b, nil
	}
	return tm + "_" + b, nil
}

type NoSumErr string

func (s NoSumErr) Error() string {
	return fmt.Sprintf("%v has no checksum to validate", string(s))
}

func IsNoSum(err error) bool {
	_, ok := err.(NoSumErr)
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
	f, err := os.Open(pic)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	_, err := io.Copy(h, f)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
