
package piclib

import (
	"fmt"
	"sync"
	"errors"
	"bytes"
	"time"
	"path"
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
	UnsupportedDir = "unsupported"
)

const (
	nameTimeFmt = "2006-01-02-15-04-05"
	currVersion = "0.1"
)

type Backend interface {
	Put(path string, data []byte) error
	Exists(path string) bool
	ListN(path string, n int) ([]string, error)
	Get(path string) ([]byte, error)
}

type Photo struct {
	Meta string
	Orig string
	Thumb1 string
	Thumb2 string
	Size int
	Uploaded time.Time
	Taken time.Time
	Tags map[string]string
	LibVersion string
}

type Library struct {
	db Backend
	seconds []Backend
	name string
	imgDir string
	thumbDir string
	indDir string
	metaDir string
	unsupportedDir string
	photoCache map[string]*Photo
	thumb1Cache map[string][]byte
	thumb2Cache map[string][]byte
	libLock sync.RWMutex
}

func New(name string, db Backend) *Library {
	return &Library{
		db: db,
		name: name,
		imgDir: path.Join(name, ImageDir),
		thumbDir: path.Join(name, ThumbDir),
		indDir: path.Join(name, IndexDir),
		metaDir: path.Join(name, MetaDir),
		unsupportedDir: path.Join(name, UnsupportedDir),
		photoCache: make(map[string]*Photo),
		thumb1Cache: make(map[string][]byte),
		thumb2Cache: make(map[string][]byte),
	}
}

func (l *Library) Close() {
}

func (l *Library) AddSecondary(db Backend) {
	l.seconds = append(l.seconds, db)
}

func (l *Library) ListPhotosN(n int) ([]string, error) {
	names, err := l.db.ListN(l.metaDir, n)
	if err != nil {
		return nil, err
	}
	bases := make([]string, len(names))
	for i, name := range names {
		bases[i] = path.Base(name)
	}
	return bases, nil
}

func (l *Library) AddPhoto(name string, data []byte) (p *Photo, err error) {
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && s == "unsupported" {
				base := path.Base(name)
				nm := base[:len(base)-len(path.Ext(name))]
				full := fmt.Sprintf("%v-sep-%v%v", time.Now().Format(nameTimeFmt), nm, path.Ext(name))
				l.putAll(l.unsupportedDir, full, data)
				err = fmt.Errorf("unsupported file type %v", name)
			} else {
				panic(r)
			}
		}
	}()

	// construct photo name
	ext := path.Ext(name)
	base := path.Base(name)
	strDate, date := dateFrom(data)
	fname := strDate + "-sep-" + base[:len(base)-len(ext)]

	// create photo meta object
	p = &Photo{
		Meta: fname + ".json",
		Orig: fname + ext,
		Thumb1: fname + "_thumb1.jpg",
		Thumb2: fname + "_thumb2.jpg",
		Size: len(data),
		Uploaded: time.Now(),
		Taken: date,
		Tags: make(map[string]string),
		LibVersion: currVersion,
	}

	/////// store all photo related data in backend ////////
	if l.db.Exists(path.Join(l.metaDir, p.Meta)) {
		return nil, errors.New("library: photo file " + p.Meta + " already exists")
	}// else if l.db.Exists(l.imgDir, p.Orig) {
	//	return nil, errors.New("library: photo file " + p.Orig + " already exists")
	//}

	// decode image bytes and construct thumbnails
	r := bytes.NewReader(data)
	img, _, err := image.Decode(r)
	if err != nil {
		panic("unsupported")
	}

	thumb1, err := thumb(144, 0, img)
	if err != nil {
		return nil, err
	}
	thumb2, err := thumb(800, 0, img)
	if err != nil {
		return nil, err
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

	l.thumb1Cache[p.Thumb1] = thumb1
	l.thumb1Cache[p.Thumb2] = thumb2
	l.photoCache[p.Meta] = p
	return p, nil
}

func (l *Library) putAll(pth, name string, data []byte) (err error) {
	fullPath := path.Join(pth, name)
	errs := []error{}
	if err = l.db.Put(fullPath, data); err != nil {
		errs = append(errs, err)
	}
	for _, second := range l.seconds {
		if err = second.Put(fullPath, data); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (l *Library) GetPhoto(name string) (*Photo, error) {
	l.libLock.RLock()
	if p, ok := l.photoCache[name]; ok {
		l.libLock.RUnlock()
		return p, nil
	}
	l.libLock.RUnlock()

	data, err := l.db.Get(path.Join(l.metaDir, name))
	if err != nil {
		return nil, err
	}

	var p Photo
	err = json.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}

	l.libLock.Lock()
	defer l.libLock.Unlock()

	l.photoCache[name] = &p
	return &p, nil
}

func (l *Library) GetOriginal(p *Photo) (data []byte, err error) {
	orig, err := l.db.Get(path.Join(l.imgDir, p.Orig))
	if err != nil {
		return nil, err
	}
	return orig, nil
}

func (l *Library) GetThumb1(p *Photo) (data []byte, err error) {
	if data, ok := l.thumb1Cache[p.Thumb1]; ok {
		return data, nil
	}

	thumb1, err := l.db.Get(path.Join(l.thumbDir, p.Thumb1))
	if err != nil {
		return nil, err
	}

	l.thumb1Cache[p.Thumb1] = thumb1
	return thumb1, nil
}

func (l *Library) GetThumb2(p *Photo) (data []byte, err error) {
	if data, ok := l.thumb2Cache[p.Thumb2]; ok {
		return data, nil
	}

	thumb2, err := l.db.Get(path.Join(l.thumbDir, p.Thumb2))
	if err != nil {
		return nil, err
	}

	l.thumb2Cache[p.Thumb1] = thumb2
	return thumb2, nil
}

func (l *Library) getIndex(name string, v interface{}) error {
	data, err := l.db.Get(path.Join(l.indDir, name))
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func dateFrom(data []byte) (string, time.Time) {
	now := time.Now()
	r := bytes.NewReader(data)
	x, err := exif.Decode(r)
	if err != nil {
		return now.Format(nameTimeFmt) + "-NoDate", now
	}

	tg, err := x.Get("DateTimeOriginal")
	if err != nil {
    tg, err = x.Get("DateTime")
    if err != nil {
      return now.Format(nameTimeFmt) + "-NoDate", now
    }
	}

	t, err := time.Parse("2006:01:02 15:04:05", tg.StringVal())
	if err != nil {
		return now.Format(nameTimeFmt) + "-NoDate", now
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

