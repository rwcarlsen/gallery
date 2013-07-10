// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/png"
	"io"
	"path"
	"time"

	"github.com/rwcarlsen/gallery/backend"
	cache "github.com/rwcarlsen/gocache"
)

const (
	Byte = 1
	Kb   = 1000 * Byte
	Mb   = 1000 * Kb
	Gb   = 1000 * Mb
)

// The library stores images and data in the listed categories with each used
// as the first part of the path when storing/retrieving such data from a
// backend.  Example, the path to a meta-data file for an image would be
// path.Join(MetaDir, metafile-name).
const (
	// ImageDir is the path to original image files.
	ImageDir = "originals"
	// MetaDir is the path to image metadata files.
	MetaDir = "metadata"
	// ThumbDir is the path to reduced-size thumbnail images.
	ThumbDir = "thumbnails"
	// UnsupportedDir is the path to files of unrecognized type that were added
	// to the Library.
	UnsupportedDir = "unsupported"
)

const (
	noDate       = "-NoDate"
	NameSep      = "-sep-"
	oldMeta      = "OldMeta"
	revSepMarker = "\n---revsepmarker---\n"
)

const (
	nameTimeFmt = "2006-01-02-15-04-05"
	Version     = "0.1"
)

// Library manages and organizes collections of Photos stored in the desired
// backend database.  Allowed image formats are those supported by Go's
// standard library image package.  Unrecognized formats of any kind (even
// non-image based) are stored in UnsupportedDir.
type Library struct {
	Db             backend.Interface
	name           string
	imgDir         string
	thumbDir       string
	metaDir        string
	unsupportedDir string
	cache          *cache.LRUCache
}

// New creates and initializes a new library.  All library data is namespaced
// under name in the backend db.  A cache of previously retrieved data is
// maintained up to cacheSize bytes in order to reduce pressure on the db
// backend.
func New(name string, db backend.Interface, cacheSize uint64) *Library {
	return &Library{
		Db:             db,
		name:           name,
		imgDir:         path.Join(name, ImageDir),
		thumbDir:       path.Join(name, ThumbDir),
		metaDir:        path.Join(name, MetaDir),
		unsupportedDir: path.Join(name, UnsupportedDir),
		cache:          cache.NewLRUCache(cacheSize),
	}
}

// ListNames the names of up to n library Photos in no particular order. The
// names can be used with the GetPhoto method for retrieving actual photo
// objects.
func (l *Library) ListNames(n int) ([]string, error) {
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

// ListPhotos is a convenience for retrieving up to n library Photos (no order
// guaruntees).  This is a convenience method covering functionality of
// ListNames and GetPhoto methods.
func (l *Library) ListPhotos(n int) ([]*Photo, error) {
	names, err := l.ListNames(n)
	if err != nil {
		return nil, err
	}

	var err2 error
	pics := make([]*Photo, 0, len(names))
	for _, name := range names {
		p, err := l.GetPhoto(name)
		if err != nil {
			err2 = fmt.Errorf("error reading metadata file '%v'", name)
			continue
		}
		pics = append(pics, p)
	}

	return pics, err2
}

// AddPhoto addes a photo to the library where name is the photo's original
// name and buf contains the entirety of the image data.  If buf contains an
// unsupported file type, the data will be stored in UnsupportedDir and an
// error returned.
func (l *Library) AddPhoto(name string, buf io.ReadSeeker) (p *Photo, err error) {
	sum, n := hash(buf)

	// create photo meta object
	p = &Photo{
		Uploaded:   time.Now(),
		Tags:       make(map[string]string),
		LibVersion: Version,
		Sha1:       sum,
		Size:       int(n),
		lib:        l,
	}
	addExifBased(buf, p, name)

	if _, err := buf.Seek(0, 0); err != nil {
		return nil, err
	}

	var thumb1, thumb2 io.ReadSeeker
	if !l.Db.Exists(path.Join(l.thumbDir, p.Thumb1)) || !l.Db.Exists(path.Join(l.thumbDir, p.Thumb2)) {
		// decode image bytes and construct thumbnails
		img, _, err := image.Decode(buf)
		if err != nil {
			// unsupported file type
			if err := l.put(l.unsupportedDir, p.Orig, buf); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("unsupported file type %v", name)
		}

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
	err = l.put(l.metaDir, p.Meta, bytes.NewReader(meta))
	if err != nil {
		err2 = err
	}

	// add photo image/thumb files to db
	err = l.put(l.imgDir, p.Orig, buf)
	if err != nil {
		err2 = err
	}

	err = l.put(l.thumbDir, p.Thumb1, thumb1)
	if err != nil {
		err2 = err
	}

	err = l.put(l.thumbDir, p.Thumb2, thumb2)
	if err != nil {
		err2 = err
	}

	return p, err2
}

// put does a safe Put into the backend database. It ensures buf read is
// seeked to the beginning and the path/name combo does not exist.
func (l *Library) put(pth, name string, buf io.ReadSeeker) (err error) {
	fullPath := path.Join(pth, name)
	if l.Db.Exists(fullPath) {
		return fmt.Errorf("piclib: photo file already exists %v", fullPath)
	}

	if _, err := buf.Seek(0, 0); err != nil {
		return err
	}
	return l.Db.Put(fullPath, buf)
}

// GetPhoto returns the named Photo from the library.
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
	p.lib = l

	l.cache.Set(name, cachePic(&p))
	return &p, nil
}

// UpdatePhoto overwrites any/all of a photo p's metadata to whatever
// new state is has been changed to.
func (l *Library) UpdatePhoto(p *Photo) error {
	pic, err := l.GetPhoto(p.Meta)
	if err != nil {
		return err
	}

	data, err := json.Marshal(pic)
	if err != nil {
		return err
	}
	return l.Db.Put(path.Join(l.metaDir, pic.Meta), bytes.NewReader(data))
}
