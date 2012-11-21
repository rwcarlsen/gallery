
package library

import (
	"path/filepath"
	"encoding/json"
	"errors"

	"github.com/rwcarlsen/gallery/photo"
)

const (
	ImageDir = "originals"
	MetaDir = "metadata"
	ThumbDir = "thumbnails"
	IndexDir = "index"
)

type Backend interface {
	Put(path, name string, data []byte) error
	Exists(path, name string) bool
	Get(path, name string) ([]byte, error)
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

func (l *Library) AddPhoto(p *photo.Photo) error {
	if l.db.Exists(l.metaDir, p.Meta) {
		return errors.New("library: photo file " + p.Meta + " already exists")
	} else if l.db.Exists(l.imgDir, p.Orig) {
		return errors.New("library: photo file " + p.Orig + " already exists")
	}

	// add photo meta-data object to db
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	err = l.putAll(l.metaDir, p.Meta, data)
	if err != nil {
		return err
	}

	// add photo image/thumb files to db
	err = l.putAll(l.imgDir, p.Orig, p.Original())
	if err != nil {
		return err
	}

	err = l.putAll(l.thumbDir, p.Thumb1, p.Thumbnail1())
	if err != nil {
		return err
	}

	err = l.putAll(l.thumbDir, p.Thumb2, p.Thumbnail2())
	if err != nil {
		return err
	}

	return nil
}

func (l *Library) putAll(path, name string, data []byte) (err error) {
	err = l.db.Put(path, name, data)
	for _, second := range l.seconds {
		err = second.Put(path, name, data)
	}
	return err
}

func (l *Library) LoadImages(p *photo.Photo) error {
	orig, err := l.db.Get(l.imgDir, p.Orig)
	if err != nil {
		return err
	}

	thumb1, err := l.db.Get(l.thumbDir, p.Thumb1)
	if err != nil {
		return err
	}

	thumb2, err := l.db.Get(l.imgDir, p.Thumb2)
	if err != nil {
		return err
	}

	p.SetImages(orig, thumb1, thumb2)
	return nil
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

		var photo *photo.Photo
		err = json.Unmarshal(data, photo)
		if err != nil {
			return nil, errors.New("corrupted photo metadata: " + err.Error())
		}
		ind.photos = append(ind.photos, photo)
	}

	return ind, nil
}

type Index struct {
	Name string
	MetaFiles []string
	photos []*photo.Photo
}

func (i *Index) Photos() []*photo.Photo {
	return i.photos
}


