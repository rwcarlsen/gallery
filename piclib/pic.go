package piclib

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Pic struct {
	id     int // for protection
	Id     int // for marshalling
	lib    *Lib
	Sum    []byte
	Name   string
	Added  time.Time
	Taken  time.Time
	Orient int // EXIF orientation (1 through 8)
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
