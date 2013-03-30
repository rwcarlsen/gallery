
package piclib

import (
	"crypto/sha1"
	"image"
	"image/jpeg"
	"strings"
	"io"
	"time"
	"path"
	"bytes"
	"fmt"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/nfnt/resize"
	cache "github.com/rwcarlsen/gocache"
)

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

