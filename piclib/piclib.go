// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"path"
	"strings"
	"time"

	"github.com/nfnt/resize"
	"github.com/rwcarlsen/gallery/backend"
	cache "github.com/rwcarlsen/gocache"
	"github.com/rwcarlsen/goexif/exif"
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
	NameSep       = "-sep-"
	oldMeta      = "OldMeta"
	revSepMarker = "\n---revsepmarker---\n"
)

const (
	nameTimeFmt = "2006-01-02-15-04-05"
	Version     = "0.1"
)

// rots holds mappings from exif oritnation tag to degrees clockwise needed
var rots = map[int]int{
	1: 0,
	2: 0,
	3: 180,
	4: 180,
	5: 90,
	6: 90,
	7: 270,
	8: 270,
}

// Photo is the object-type managed by the library.  It provides methods for
// retrieving photo-related information from the Library as well as defines the
// photo metadata schema.
// Photos usually should not be created manually - rather they should be
// created through the Library's AddPhoto method.
type Photo struct {
	Meta        string
	Orig        string
	Thumb1      string
	Thumb2      string
	Size        int
	Uploaded    time.Time
	Taken       time.Time
	Orientation int
	Tags        map[string]string
	LibVersion  string
	Sha1	    string
	lib         *Library
}

// LegitTaken returns true only if this photo's Taken date was retrieved from
// existing EXIF data embedded in the image.
func (p *Photo) LegitTaken() bool {
	return !strings.Contains(p.Meta, noDate)
}

// GetOriginal retrieves the data for the photo's original, full-resolution
// image.  Returns an error if the photo was neither created nor retrieved from
// a Library. Other retrieval errors may be returned.
func (p *Photo) GetOriginal() (data []byte, err error) {
	if p.lib == nil {
		return nil, errors.New("piclib: photo not initialized with library")
	}
	orig, err := p.lib.Db.Get(path.Join(p.lib.imgDir, p.Orig))
	if err != nil {
		return nil, err
	}
	return orig, nil
}

// GetThumb1 retrieves the data for the photo's large thumbnail image (suitable
// for online sharing).  Returns an error if the photo was neither created nor
// retrieved from a Library. Other retrieval errors may be returned.
func (p *Photo) GetThumb1() (data []byte, err error) {
	if p.lib == nil {
		return nil, errors.New("piclib: photo not initialized with library")
	}
	if v, ok := p.lib.cache.Get(p.Thumb1); ok {
		return v.(*cacheVal).data, nil
	}

	thumb1, err := p.lib.Db.Get(path.Join(p.lib.thumbDir, p.Thumb1))
	if err != nil {
		return nil, err
	}

	p.lib.cache.Set(p.Thumb1, cacheData(thumb1))
	return thumb1, nil
}

// GetThumb2 retrieves the data for the photo's small thumbnail image (suitable
// for grid-views, etc).  Returns an error if the photo was neither created nor
// retrieved from a Library. Other retrieval errors may be returned.
func (p *Photo) GetThumb2() (data []byte, err error) {
	if p.lib == nil {
		return nil, errors.New("piclib: photo not initialized with library")
	}
	if v, ok := p.lib.cache.Get(p.Thumb2); ok {
		return v.(*cacheVal).data, nil
	}

	thumb2, err := p.lib.Db.Get(path.Join(p.lib.thumbDir, p.Thumb2))
	if err != nil {
		return nil, err
	}

	p.lib.cache.Set(p.Thumb2, cacheData(thumb2))
	return thumb2, nil
}

// Rotation returns the number of degrees clockwise the photo must be
// rotated to be right-side-up.
func (p *Photo) Rotation() int {
	return rots[p.Orientation]
}

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

	errString := ""
	pics := make([]*Photo, 0, len(names))
	for _, name := range names {
		p, err := l.GetPhoto(name)
		if err != nil {
			errString += "\n" + err.Error()
			continue
		}
		pics = append(pics, p)
	}

	if len(errString) > 0 {
		return pics, errors.New("piclib: " + errString)
	}
	return pics, nil
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

// put does a safe Put into the backend database (e.g. ensures buf read is set
// to beginning).
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

func addExifBased(buf io.ReadSeeker, p *Photo, origName string) {
	name := ""

	date, orient := exifFrom(buf)

	t, err := time.Parse("2006:01:02 15:04:05", date)
	if err != nil {
		t = time.Now()
		name += p.Sha1 + noDate
	} else {
		name += t.Format(nameTimeFmt)
	}

	p.Orientation = orient
	p.Taken = t

	ext := strings.ToLower(path.Ext(origName))
	base := path.Base(origName)
	name += NameSep + base[:len(base)-len(ext)]

	p.Meta = name + ".json"
	p.Orig = name + ext
	p.Thumb1 = name + "_thumb1.jpg"
	p.Thumb2 = name + "_thumb2.jpg"
}

func exifFrom(buf io.ReadSeeker) (date string, orient int) {
	orient = 1
	if _, err := buf.Seek(0, 0); err != nil {
		return noDate, orient
	}

	x, err := exif.Decode(buf)
	if err != nil {
		return noDate, orient
	}

	tg, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		if tg, err = x.Get(exif.DateTime); err != nil {
			date = noDate
		}
	}
	if tg != nil {
		date = tg.StringVal()
	}

	if tg, err := x.Get(exif.Orientation); err == nil {
		orient = int(tg.Int(0))
	}
	return date, orient
}

// thumb returns a shrunken version of an image.
func thumb(w, h uint, img image.Image) (io.ReadSeeker, error) {
	m := resize.Resize(w, h, img, resize.Bilinear)

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, m, nil)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf.Bytes()), nil
}

// cacheVal allows Library-related data to be placed in a size-limited cache.
type cacheVal struct {
	size int
	data []byte
	p    *Photo
}

func (cv *cacheVal) Size() int {
	return cv.size
}

// cachePic creates a cacheable value from a Photo (meta-data).
func cachePic(p *Photo) cache.Value {
	return &cacheVal{
		p:    p,
		size: 2000,
	}
}

// cacheData creates a cacheable value from a byte-slice.
func cacheData(data []byte) cache.Value {
	return &cacheVal{
		data: data,
		size: len(data),
	}
}

// hash returns a hex string representing the sha1 hash-sum of the file and
// the number of bytes hashed.
func hash(r io.ReadSeeker) (sum string, n int64) {
	r.Seek(0, 0)
	h := sha1.New()
	var err error
	if n, err = io.Copy(h, r); err != nil {
		return "FailedHash", n
	}
	return fmt.Sprintf("%X", h.Sum(nil)), n
}

