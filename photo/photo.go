
package photo

import (
	"bytes"
	"path/filepath"
	"time"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
)

const (
	timeFormat = "2006-01-02-15-04-05"
)

type Image []byte

type Photo struct {
	Meta string
	Orig string
	Thumb1 string
	Thumb2 string
	Uploaded time.Time
	Taken time.Time
	Tags map[string]string
	orig Image
	thumb1 Image
	thumb2 Image
}

func dateFrom(data []byte) (string, time.Time) {
	now := time.Now()
	r := bytes.NewReader(data)
	x, err := exif.Decode(r)
	if err != nil {
		return "NONE-" + now.Format(timeFormat), now
	}
	tg, err := x.Get("DateTimeOriginal")
	if err != nil {
		return "NONE-" + now.Format(timeFormat), now
	}

	t, err := time.Parse("2006:01:02 15:04:05", tg.StringVal())
	if err != nil {
		return "NONE-" + now.Format(timeFormat), now
	}

	return t.Format(timeFormat), t
}

func New(name string, data []byte) (*Photo, error) {
	ext := filepath.Ext(name)
	base := filepath.Base(name)
	strDate, date := dateFrom(data)
	fname := strDate + "-" + base[:len(base)-len(ext)]

	r := bytes.NewReader(data)
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	thumb1, err := thumb(144, 0, img)
	if err != nil {
		return nil, err
	}
	thumb2, err := thumb(144, 0, img)
	if err != nil {
		return nil, err
	}

	return &Photo{
		Meta: fname + ".meta",
		Orig: fname + ext,
		Thumb1: fname + "_thumb1.jpg",
		Thumb2: fname + "_thumb2.jpg",
		Uploaded: time.Now(),
		Taken: date,
		Tags: make(map[string]string),
		orig: data,
		thumb1: thumb1,
		thumb2: thumb2,
	}, nil
}

func (p *Photo) SetImages(orig, thumb1, thumb2 []byte) {
	p.orig = orig
	p.thumb1 = thumb1
	p.thumb2 = thumb2
}

func (p *Photo) Original() []byte {
	return []byte(p.orig)
}

func (p *Photo) Thumbnail1() []byte {
	return []byte(p.thumb1)
}

func (p *Photo) Thumbnail2() []byte {
	return []byte(p.thumb2)
}

func thumb(w, h int, img image.Image) ([]byte, error) {
	m := resize.Resize(144, 0, img, resize.Lanczos3)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, m, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
