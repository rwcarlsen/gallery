package piclib

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
	"github.com/nfnt/resize"
)

func DefaultPath() string {
	s := os.Getenv("PICLIB")
	if s != "" {
		return s
	}
	return filepath.Join(os.Getenv("HOME"), ".piclib")
}

func Sha256(r io.Reader) (sum []byte, err error) {
	h := sha256.New()
	_, err = io.Copy(h, r)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func MakeThumb(r io.Reader, w, h int, orient int) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	m := resize.Resize(uint(w), uint(h), img, resize.Bicubic)

	switch orient {
	case 3, 4:
		m = imaging.Rotate180(m)
	case 5, 6:
		m = imaging.Rotate270(m)
	case 7, 8:
		m = imaging.Rotate90(m)
	}

	switch orient {
	case 2, 5, 4, 7:
		m = imaging.FlipH(m)
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, m, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func LoadStream(l *Lib, r io.Reader) ([]*Pic, error) {
	dec := json.NewDecoder(bufio.NewReader(r))
	var err error
	pics := []*Pic{}
	for {
		p := &Pic{}
		err = dec.Decode(&p)
		if err != nil {
			break
		}
		preal, err := l.Open(p.Id)
		if err != nil {
			return nil, err
		}
		pics = append(pics, preal)
	}
	if err != io.EOF {
		return nil, err
	}
	return pics, nil
}
