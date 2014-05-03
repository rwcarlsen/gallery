// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	_ "image/gif"
	_ "image/png"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

const Version = "0.1"

const (
	NoDate      = "nodate-sha1-"
	NameSep     = "-sep-"
	NotesExt    = ".notes"
	nameTimeFmt = "2006-01-02-15-04-05"
)

type Meta struct {
	Sha1 string
}

var Path = filepath.Join(os.Getenv("HOME"), "piclib")

func init() {
	s := os.Getenv("PICLIB")
	if len(s) > 0 {
		Path = s
	}
}

type DupErr string

func (s DupErr) Error() string {
	return fmt.Sprintf("%v already exists in library", string(s))
}

func IsDup(err error) bool {
	_, ok := err.(DupErr)
	return ok
}

func Add(pic string, rename bool) (newname string, err error) {
	// check if pic path is already within library path
	if abs, err := filepath.Abs(pic); err != nil {
		return "", err
	} else if strings.HasPrefix(abs, Path) {
		return pic, nil
	}

	// check if dst path exists
	dstpath := filepath.Join(Path, filepath.Base(pic))
	if f, err := os.Open(dstpath); err == nil {
		f.Close()
		return "", DupErr(pic)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	dst, err := os.Create(dstpath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	src, err := os.Open(pic)
	if err != nil {
		return "", err
	}
	defer src.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return "", err
	}

	if rename {
		return Rename(dstpath)
	}
	return pic, nil
}

func Rename(pic string) (newname string, err error) {
	name, err := CanonicalName(pic)
	if err != nil {
		return "", err
	} else if name == pic {
		return pic, nil
	}
	err = os.Rename(pic, name)
	if err != nil {
		return "", err
	}
	return name, nil
}

func CanonicalName(pic string) (string, error) {
	dir := filepath.Dir(pic)
	b := filepath.Base(pic)

	t := Taken(pic)
	tm := t.Format(nameTimeFmt)
	if t.IsZero() {
		sum, err := Checksum(pic)
		if err != nil {
			return "", err
		}
		tm = fmt.Sprintf("%x", sum)
		return filepath.Join(dir, NoDate+tm+NameSep+b), nil
	}
	return filepath.Join(dir, tm+NameSep+b), nil
}

func Taken(pic string) time.Time {
	f, err := os.Open(pic)
	if err != nil {
		return time.Time{}
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}
	}

	tg, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		if tg, err = x.Get(exif.DateTime); err != nil {
			return time.Time{}
		}
	}
	if tg == nil {
		return time.Time{}
	}

	t, err := time.Parse("2006:01:02 15:04:05", tg.StringVal())
	if err != nil {
		return time.Time{}
	}
	return t
}

func NotesPath(pic string) string {
	return pic + NotesExt
}

func Notes(pic string) (notes string, m *Meta, err error) {
	data, err := ioutil.ReadFile(NotesPath(pic))
	if os.IsNotExist(err) {
		return "", &Meta{}, nil
	} else if err != nil {
		return "", nil, err
	}

	notes = string(data)

	buf := bytes.NewBuffer(data)
	dec := json.NewDecoder(buf)
	m = &Meta{}
	if err := dec.Decode(&m); err == nil {
		data, err := ioutil.ReadAll(dec.Buffered())
		if err != nil {
			return "", nil, err
		}
		notes = string(data)
	} else {
		m = nil
	}

	return notes, m, nil
}

func WriteMeta(pic string, m *Meta) error {
	notes, _, err := Notes(pic)
	if err != nil {
		return err
	}

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(NotesPath(pic), append([]byte(notes), data...), 0644)
}

func Checksum(pic string) ([]byte, error) {
	f, err := os.Open(pic)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func SaveChecksum(pic string) error {
	_, meta, err := Notes(pic)
	if err != nil {
		return err
	} else if meta != nil && len(meta.Sha1) > 0 {
		return nil
	}

	sum, err := Checksum(pic)
	if err != nil {
		return err
	}

	meta.Sha1 = fmt.Sprintf("%x", sum)
	return WriteMeta(pic, meta)
}

func Validate(pic string) error {
	_, meta, err := Notes(pic)
	if err != nil {
		return err
	} else if meta == nil || len(meta.Sha1) == 0 {
		return fmt.Errorf("%v has no checksum to validate", pic)
	}

	sum, err := Checksum(pic)
	if err != nil {
		return err
	}

	if meta.Sha1 != fmt.Sprintf("%x", sum) {
		return fmt.Errorf("%v failed checksum validation")
	}

	return nil
}
