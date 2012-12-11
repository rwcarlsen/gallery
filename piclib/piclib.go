package piclib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
	"path"
	"strings"
	"time"
	"io"

	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
	cache "github.com/rwcarlsen/gocache"
)

const (
	Byte = 1
	Kb = 1000 * Byte
	Mb = 1000 * Kb
	Gb = 1000 * Mb
)

const (
	ImageDir       = "originals"
	MetaDir        = "metadata"
	ThumbDir       = "thumbnails"
	IndexDir       = "index"
	UnsupportedDir = "unsupported"
)

const (
	noDate = "-NoDate"
)

const (
	nameTimeFmt = "2006-01-02-15-04-05"
	currVersion = "0.1"
)

type Backend interface {
	Put(path string, r io.ReadSeeker) error
	Exists(path string) bool
	ListN(path string, n int) ([]string, error)
	Get(path string) ([]byte, error)
	Name() string
}

type Photo struct {
	Meta       string
	Orig       string
	Thumb1     string
	Thumb2     string
	Size       int
	Uploaded   time.Time
	Taken      time.Time
	Tags       map[string]string
	LibVersion string
}

func (p *Photo) LegitTaken() bool {
	return !strings.Contains(p.Meta, noDate)
}

type Library struct {
	Db             Backend
	name           string
	imgDir         string
	thumbDir       string
	indDir         string
	metaDir        string
	unsupportedDir string
	cache		   *cache.LRUCache
}

func New(name string, db Backend, cacheSize uint64) *Library {
	return &Library{
		Db:             db,
		name:           name,
		imgDir:         path.Join(name, ImageDir),
		thumbDir:       path.Join(name, ThumbDir),
		indDir:         path.Join(name, IndexDir),
		metaDir:        path.Join(name, MetaDir),
		unsupportedDir: path.Join(name, UnsupportedDir),
		cache: cache.NewLRUCache(cacheSize),
	}
}

func (l *Library) ListPhotosN(n int) ([]string, error) {
	names, err := l.Db.ListN(l.metaDir, n)
	if err != nil {
		return nil, err
	}
	bases := make([]string, len(names))
	for i, name := range names {
		bases[i] = path.Base(name)
	}
	return bases, nil
}

func (l *Library) AddPhoto(name string, buf io.ReadSeeker) (p *Photo, err error) {
	defer func() {
		if r := recover(); r != nil {
			base := path.Base(name)
			nm := base[:len(base)-len(path.Ext(name))]
			if s, ok := r.(string); ok && s == "unsupported" {
				full := fmt.Sprintf("%v-sep-unsupported-%v%v", time.Now().Format(nameTimeFmt), nm, path.Ext(name))
				l.putAll(l.unsupportedDir, full, buf)
				err = fmt.Errorf("unsupported file type %v", name)
			} else {
				panic(r)
				full := fmt.Sprintf("%v-sep-badfile-%v%v", time.Now().Format(nameTimeFmt), nm, path.Ext(name))
				l.putAll(l.unsupportedDir, full, buf)
				err = fmt.Errorf("corrupt file %v: %v", name, r)
			}
		}
	}()

	// construct photo name
	ext := path.Ext(name)
	base := path.Base(name)
	strDate, date := dateFrom(buf)
	fname := strDate + "-sep-" + base[:len(base)-len(ext)]

	// create photo meta object
	p = &Photo{
		Meta:       fname + ".json",
		Orig:       fname + strings.ToLower(ext),
		Thumb1:     fname + "_thumb1.jpg",
		Thumb2:     fname + "_thumb2.jpg",
		Uploaded:   time.Now(),
		Taken:      date,
		Tags:       make(map[string]string),
		LibVersion: currVersion,
	}

	if _, err := buf.Seek(0, 0); err != nil {
		return nil, err
	}

	// decode image bytes and construct thumbnails
	img, _, err := image.Decode(buf)
	if err != nil {
		fmt.Println("--------------------------------- ", err)
		panic("unsupported")
	}

	var thumb1, thumb2 io.ReadSeeker
	if !l.Db.Exists(path.Join(l.thumbDir, p.Thumb1)) || !l.Db.Exists(path.Join(l.thumbDir, p.Thumb2)) {
		thumb1, err = thumb(144, 0, img)
		if err != nil {
			return nil, err
		}
		thumb2, err = thumb(800, 0, img)
		if err != nil {
			return nil, err
		}
	}
	/////// store all photo related data in backend ////////

	var err2 error
	// add photo meta-data object to db
	meta, err := json.Marshal(p)
	if err != nil {
		err2 = err
	}
	err = l.putAll(l.metaDir, p.Meta, bytes.NewReader(meta))
	if err != nil {
		err2 = err
	}

	// add photo image/thumb files to db
	err = l.putAll(l.imgDir, p.Orig, buf)
	if err != nil {
		err2 = err
	}

	err = l.putAll(l.thumbDir, p.Thumb1, thumb1)
	if err != nil {
		err2 = err
	}

	err = l.putAll(l.thumbDir, p.Thumb2, thumb2)
	if err != nil {
		err2 = err
	}

	return p, err2
}

func (l *Library) putAll(pth, name string, buf io.ReadSeeker) (err error) {
	fullPath := path.Join(pth, name)
	if l.Db.Exists(fullPath) {
		return fmt.Errorf("piclib: photo file already exists %v", fullPath)
	}

	if _, err := buf.Seek(0, 0); err != nil {
		return err
	}
	return l.Db.Put(fullPath, buf)
}

type cacheVal struct {
	size int
	data []byte
	p *Photo
}

func CachePic(p *Photo) cache.Value {
	return &cacheVal{
		p: p,
		size: 2000,
	}
}

func CacheData(data []byte) cache.Value {
	return &cacheVal{
		data: data,
		size: len(data),
	}
}

func (cv *cacheVal) Size() int {
	return cv.size
}

func (l *Library) GetPhoto(name string) (*Photo, error) {
	if v, ok := l.cache.Get(name); ok {
		return v.(*cacheVal).p, nil
	}

	data, err := l.Db.Get(path.Join(l.metaDir, name))
	if err != nil {
		return nil, err
	}

	var p Photo
	err = json.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}


	l.cache.Set(name, CachePic(&p))
	return &p, nil
}

func (l *Library) GetOriginal(p *Photo) (data []byte, err error) {
	orig, err := l.Db.Get(path.Join(l.imgDir, p.Orig))
	if err != nil {
		return nil, err
	}
	return orig, nil
}

func (l *Library) GetThumb1(p *Photo) (data []byte, err error) {
	if v, ok := l.cache.Get(p.Thumb1); ok {
		return v.(*cacheVal).data, nil
	}

	thumb1, err := l.Db.Get(path.Join(l.thumbDir, p.Thumb1))
	if err != nil {
		return nil, err
	}

	l.cache.Set(p.Thumb1, CacheData(thumb1))
	return thumb1, nil
}

func (l *Library) GetThumb2(p *Photo) (data []byte, err error) {
	if v, ok := l.cache.Get(p.Thumb2); ok {
		return v.(*cacheVal).data, nil
	}

	thumb2, err := l.Db.Get(path.Join(l.thumbDir, p.Thumb2))
	if err != nil {
		return nil, err
	}

	l.cache.Set(p.Thumb2, CacheData(thumb2))
	return thumb2, nil
}

func (l *Library) getIndex(name string, v interface{}) error {
	data, err := l.Db.Get(path.Join(l.indDir, name))
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func dateFrom(buf io.Reader) (string, time.Time) {
	now := time.Now()
	x, err := exif.Decode(buf)
	if err != nil {
		return now.Format(nameTimeFmt) + noDate, now
	}

	tg, err := x.Get("DateTimeOriginal")
	if err != nil {
		tg, err = x.Get("DateTime")
		if err != nil {
			return now.Format(nameTimeFmt) + noDate, now
		}
	}

	t, err := time.Parse("2006:01:02 15:04:05", tg.StringVal())
	if err != nil {
		return now.Format(nameTimeFmt) + noDate, now
	}

	return t.Format(nameTimeFmt), t
}

func thumb(w, h uint, img image.Image) (io.ReadSeeker, error) {
	m := resize.Resize(w, h, img, resize.Bilinear)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, m, nil)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}
