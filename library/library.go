
package library

import (
	"github.com/rwcarlsen/gallery/photo"
)

const (
	ImageDir = "photos"
	MetaDir = "meta"
	ThumbDir = "thumbs"
	IndexDir = "index"
)

type Backend interface {
	Put(path, name string, data []byte) error
	Get(path, name string) ([]byte, error)
}

type Library struct {
	db Backend
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

func (l *Library) AddPhoto(p *Photo) error {
	// add photo meta-data object to db
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	err = l.db.Put(l.metaDir, , p.Original())
	if err != nil {
		return err
	}

	// add photo image/thumb files to db
	err = l.db.Put(l.imgDir, p.Orig, p.Original())
	if err != nil {
		return err
	}

	err = l.db.Put(l.thumbDir, p.Thumb1, p.Thumbnail1())
	if err != nil {
		return err
	}

	err = l.db.Put(l.thumbDir, p.Thumb2, p.Thumbnail2())
	if err != nil {
		return err
	}
}

func (l *Library) LoadPhoto(p *Photo) error {
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

	p.LoadImages(orig, thumb1, thumb2)
}

func (l *Library) GetIndex(index string) (*Index, error) {
	thumb2, err := l.db.Get(l.indDir, index)
}

type Index struct {
	photos []*Photo
}
