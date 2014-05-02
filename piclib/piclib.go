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
)

const Version = "0.1"

const (
	NoDate      = "-NoDate"
	NameSep     = "-sep-"
	NotesExt    = ".notes"
	SumPrefix   = "sha1:"
	nameTimeFmt = "2006-01-02-15-04-05"
)

type Meta struct {
	Sha1 string
}

func Stem(path string) string {
	s := filepath.Base(path)
	return s[:len(s)-len(filepath.Ext(s))]
}

func Notes(pic string) (notes string, m *Meta, err error) {
	p := filepath.Join(filepath.Dir(pic), Stem(pic)+NotesExt)
	data, err := ioutil.ReadFile(p)
	if os.IsNotExist(err) {
		return "", &Meta{}, nil
	} else if err != nil {
		return "", nil, err
	}

	notes = string(data)

	buf := bytes.NewBuffer(data)
	dec := json.NewDecoder(buf)
	m := &Meta{}
	if err := dec.Decode(&m); err == nil {
		data, err := ioutil.ReadAll(dec.Buffered())
		if err != nil {
			return "", nil, err
		}
		notes = string(data)
	}

	return notes, m, nil
}

func WriteMeta(pic, m *Meta) error {
	notes, meta, err := Notes(pic)
	if err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	p := filepath.Join(filepath.Dir(pic), Stem(pic)+NotesExt)
	return ioutil.WriteFile(p, append([]byte(notes), data...), 0755)
}

func WriteNotes(pic, notes string) error {
	_, meta, err := Notes(pic)
	if err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	p := filepath.Join(filepath.Dir(pic), Stem(pic)+NotesExt)
	return ioutil.WriteFile(p, append([]byte(notes), data...), 0755)
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(notes)
	return err
}

func SaveChecksum(pic string) error {
	notes, meta, err := Notes(pic)
	if err != nil {
		return err
	} else if len(meta.Sha1) > 0 {
		return nil
	}

	f, err := os.Open(pic)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha1.New()
	_, err := io.Copy(h, f)
	if err != nil {
		return nil
	}
	meta.Sha1 := fmt.Sprintf("%x", sha1.Sum(nil))

	notes = fmt.Sprintf("%s%x%s\n", SumPrefix, sum, notes)
	return WriteNotes(pic, notes, true)
}
