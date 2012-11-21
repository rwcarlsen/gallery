
package library

import (
	"errors"
	"bytes"
	"time"
	"path/filepath"
	"encoding/json"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"

	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

const (
	ImageDir = "originals"
	MetaDir = "metadata"
	ThumbDir = "thumbnails"
	IndexDir = "index"
)

const (
	nameTimeFmt = "2006-01-02-15-04-05"
)

type Backend interface {
	Put(path, name string, data []byte) error
	Exists(path, name string) bool
	Get(path, name string) ([]byte, error)
}

type Photo struct {
	Meta string
	Orig string
	Thumb1 string
	Thumb2 string
	Uploaded time.Time
	Taken time.Time
	Tags map[string]string
}

type Library struct {
	db Backend
	seconds []Backend
	name string
	imgDir string
	thumbDir string
	indDir string
	metaDir string
}

func New(name string, db Backend) *Library {
	return &Library{
		db: db,
		name: name,
		imgDir: filepath.Join(name, ImageDir),
		thumbDir: filepath.Join(name, ThumbDir),
		indDir: filepath.Join(name, IndexDir),
		metaDir: filepath.Join(name, MetaDir),
	}
}

func (l *Library) AddSecondary(db Backend) {
	l.seconds = append(l.seconds, db)
}

func (l *Library) AddPhoto(name string, data []byte) (*Photo, error) {
	// construct photo name
	ext := filepath.Ext(name)
	base := filepath.Base(name)
	strDate, date := dateFrom(data)
	fname := strDate + "-" + base[:len(base)-len(ext)]

	// decode image bytes and construct thumbnails
	r := bytes.NewReader(data)
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, err
	}

	thumb1, err := thumb(144, 0, img)
	if err != nil {
		return nil, err
	}
	thumb2, err := thumb(800, 0, img)
	if err != nil {
		return nil, err
	}

	// create photo meta object
	p := &Photo{
		Meta: fname + ".json",
		Orig: fname + ext,
		Thumb1: fname + "_thumb1.jpg",
		Thumb2: fname + "_thumb2.jpg",
		Uploaded: time.Now(),
		Taken: date,
		Tags: make(map[string]string),
	}

	/////// store all photo related data in backend ////////
	if l.db.Exists(l.metaDir, p.Meta) {
		return nil, errors.New("library: photo file " + p.Meta + " already exists")
	} else if l.db.Exists(l.imgDir, p.Orig) {
		return nil, errors.New("library: photo file " + p.Orig + " already exists")
	}

	// add photo meta-data object to db
	meta, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	err = l.putAll(l.metaDir, p.Meta, meta)
	if err != nil {
		return nil, err
	}

	// add photo image/thumb files to db
	err = l.putAll(l.imgDir, p.Orig, data)
	if err != nil {
		return nil, err
	}

	err = l.putAll(l.thumbDir, p.Thumb1, thumb1)
	if err != nil {
		return nil, err
	}

	err = l.putAll(l.thumbDir, p.Thumb2, thumb2)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (l *Library) putAll(path, name string, data []byte) (err error) {
	err = l.db.Put(path, name, data)
	for _, second := range l.seconds {
		err = second.Put(path, name, data)
	}
	return err
}

func (l *Library) GetOriginal(p *Photo) (data []byte, err error) {
	orig, err := l.db.Get(l.imgDir, p.Orig)
	if err != nil {
		return nil, err
	}
	return orig, nil
}

func (l *Library) GetThumb1(p *Photo) (data []byte, err error) {
	thumb1, err := l.db.Get(l.thumbDir, p.Thumb1)
	if err != nil {
		return nil, err
	}
	return thumb1, nil
}

func (l *Library) GetThumb2(p *Photo) (data []byte, err error) {
	thumb2, err := l.db.Get(l.imgDir, p.Thumb2)
	if err != nil {
		return nil, err
	}
	return thumb2, nil
}

func (l *Library) GetIndex(index string) (*Index, error) {
	data, err := l.db.Get(l.indDir, index)
	if err != nil {
		return nil, err
	}

	var ind *Index
	err = json.Unmarshal(data, ind)
	if err != nil {
		return nil, errors.New("library: malformed index - " + err.Error())
	}
	for _, name := range ind.MetaFiles {
		data, err := l.db.Get(l.metaDir, name)
		if err != nil {
			return nil, errors.New("corrupted photo index or missing photos: " + err.Error())
		}

		var photo *Photo
		err = json.Unmarshal(data, photo)
		if err != nil {
			return nil, errors.New("corrupted photo metadata: " + err.Error())
		}
		ind.photos = append(ind.photos, photo)
	}

	return ind, nil
}

func dateFrom(data []byte) (string, time.Time) {
	now := time.Now()
	r := bytes.NewReader(data)
	x, err := exif.Decode(r)
	if err != nil {
		return "NONE-" + now.Format(nameTimeFmt), now
	}
	tg, err := x.Get("DateTimeOriginal")
	if err != nil {
		return "NONE-" + now.Format(nameTimeFmt), now
	}

	t, err := time.Parse("2006:01:02 15:04:05", tg.StringVal())
	if err != nil {
		return "NONE-" + now.Format(nameTimeFmt), now
	}

	return t.Format(nameTimeFmt), t
}

func thumb(w, h uint, img image.Image) ([]byte, error) {
	m := resize.Resize(w, h, img, resize.Bilinear)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, m, nil)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type Index struct {
	Name string
	MetaFiles []string
	photos []*Photo
}

func (i *Index) Photos() []*Photo {
	return i.photos
}

