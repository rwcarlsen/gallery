
package piclib

import (
	"time"
	"strings"
	"errors"
	"path"
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
