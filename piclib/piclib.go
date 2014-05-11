// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nfnt/resize"
	"github.com/rwcarlsen/goexif/exif"
)

const Version = "0.1"

const (
	NoDate      = "nodate-sha1-"
	NotesExt    = ".notes"
	ThumbExt    = ".thumb"
	nameTimeFmt = "2006-01-02-15-04-05"
)

// rots holds mappings from exif orientation tag to degrees clockwise needed
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

type Meta struct {
	Sha1   string
	Taken  time.Time
	Orient int
}

var Path = filepath.Join(os.Getenv("HOME"), "piclib")

func init() {
	s := os.Getenv("PICLIB")
	if len(s) > 0 {
		Path = s
	}
}

type DupErr string

func (s DupErr) Error() string {
	return fmt.Sprintf("%v already exists in library", string(s))
}

func IsDup(err error) bool {
	_, ok := err.(DupErr)
	return ok
}

func Filepath(pic string) string {
	p := filepath.Base(pic)
	if strings.HasSuffix(p, NotesExt) {
		p = p[:len(p)-len(NotesExt)]
	} else if strings.HasSuffix(p, ThumbExt) {
		p = p[:len(p)-len(ThumbExt)]
	}
	return filepath.Join(Path, p)
}

func List(n int, skipext ...string) (pics []string, err error) {
	f, err := os.Open(Path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	names, err := f.Readdirnames(n)
	if err != nil {
		return nil, err
	}

	skipext = append(skipext, NotesExt)
	skipext = append(skipext, ThumbExt)
	paths := []string{}
	for _, name := range names {
		if strings.HasPrefix(name, ".") {
			continue
		} else if fi, err := os.Stat(filepath.Join(Path, name)); err == nil && fi.IsDir() {
			continue
		}

		skip := false
		for _, ext := range skipext {
			if strings.ToLower(filepath.Ext(name)) == ext {
				skip = true
				break
			}
		}
		if !skip {
			paths = append(paths, filepath.Join(Path, name))
		}
	}
	return paths, nil
}

// Add copies a picture in the Path directory.  If rename is true, the copied
// file is renamed to CanonicalName(pic).
func Add(pic string, rename bool) (newname string, err error) {
	// make pic lib dir if it doesn't exist
	if err := os.MkdirAll(Path, 0755); err != nil {
		return "", err
	}

	// check if pic path is already within library path
	if abs, err := filepath.Abs(pic); err != nil {
		return "", err
	} else if strings.HasPrefix(abs, Path) {
		return pic, nil
	}

	// check if dst path exists
	canon, err := CanonicalName(pic)
	if err != nil {
		return "", err
	}

	dstpath := filepath.Join(Path, filepath.Base(pic))
	if rename {
		name, err := CanonicalName(pic)
		if err != nil {
			fmt.Println("spot1")
			return "", err
		}
		dstpath = filepath.Join(Path, name)
	}
	if _, err := os.Stat(dstpath); err == nil {
		return "", DupErr(pic)
	} else if !os.IsNotExist(err) {
		fmt.Println("spot2")
		return "", err
	} else if _, err := os.Stat(filepath.Join(Path, canon)); err == nil {
		return "", DupErr(pic)
	} else if !os.IsNotExist(err) {
		fmt.Println("spot3")
		return "", err
	}

	dst, err := os.Create(dstpath)
	if err != nil {
		fmt.Println("spot4")
		return "", err
	}
	defer dst.Close()

	src, err := os.Open(pic)
	if err != nil {
		fmt.Println("spot5")
		return "", err
	}
	defer src.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		fmt.Println("spot6")
		return "", err
	}

	return filepath.Base(dstpath), nil
}

func Rename(pic string) (newname string, err error) {
	name, err := CanonicalName(pic)
	if err != nil {
		return "", err
	} else if name == pic {
		return pic, nil
	}
	err = os.Rename(pic, name)
	if err != nil {
		return "", err
	}
	return name, nil
}

func CanonicalName(pic string) (string, error) {
	b := filepath.Base(pic)
	if i := strings.Index(b, "-sep-"); i != -1 {
		b = b[i+len("-sep-"):]
	}

	t := Taken(pic)

	tm := t.Format(nameTimeFmt)
	if t.IsZero() {
		sum, err := Checksum(pic)
		if err != nil {
			return "", err
		}
		tm = fmt.Sprintf("%x", sum)
		return NoDate + tm + b, nil
	}
	return tm + "_" + b, nil
}

// Taken returns the date taken of the given pic path.  No library searching -
// pic must be a correct filepath.
func Taken(pic string) time.Time {
	// use meta data date taken if it exists
	if _, meta, err := Notes(pic); err == nil && meta != nil {
		if !meta.Taken.IsZero() {
			return meta.Taken
		}
	}

	f, err := os.Open(pic)
	if err != nil {
		return time.Time{}
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}
	}

	tg, err := x.Get(exif.DateTimeOriginal)
	if err != nil {
		if tg, err = x.Get(exif.DateTime); err != nil {
			return time.Time{}
		}
	}
	if tg == nil {
		return time.Time{}
	}

	t, err := time.Parse("2006:01:02 15:04:05", tg.StringVal())
	if err != nil {
		return time.Time{}
	}
	return t
}

func Orientation(pic string) int {
	// use meta data orientation if it exists
	if _, meta, err := Notes(pic); err == nil && meta != nil {
		if meta.Orient != 0 {
			return meta.Orient
		}
	}

	f, err := os.Open(Filepath(pic))
	if err != nil {
		return 0
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return 0
	}

	tg, err := x.Get(exif.Orientation)
	if err != nil {
		return 0
	}
	return rots[int(tg.Int(0))]
}

func NotesPath(pic string) string {
	return Filepath(pic) + NotesExt
}

func ThumbPath(pic string) string {
	return Filepath(pic) + ThumbExt
}

// ThumbFile returns the thumnail filepath for pic if it exists, otherwise,
// returns the pic filepath.
func ThumbFile(pic string) string {
	if _, err := os.Stat(ThumbPath(pic)); err != nil {
		return Filepath(pic)
	}
	return ThumbPath(pic)
}

func Notes(pic string) (notes string, m *Meta, err error) {
	data, err := ioutil.ReadFile(NotesPath(pic))
	if os.IsNotExist(err) {
		return "", &Meta{}, nil
	} else if err != nil {
		return "", nil, err
	}

	notes = string(data)

	buf := bytes.NewBuffer(data)
	dec := json.NewDecoder(buf)
	m = &Meta{}
	if err := dec.Decode(&m); err == nil {
		data, err := ioutil.ReadAll(dec.Buffered())
		if err != nil {
			return "", nil, err
		}
		notes = string(data)
	} else {
		m = nil
	}

	return notes, m, nil
}

func WriteNotes(pic string, notes string) error {
	_, meta, err := Notes(pic)
	if err != nil {
		return err
	}

	data := []byte{}
	if meta != nil {
		data, err = json.Marshal(meta)
		if err != nil {
			return err
		}
		data = append(data, '\n')
	}

	err = ioutil.WriteFile(NotesPath(pic), append(data, []byte(notes)...), 0644)
	if err != nil {
		return err
	}
	return nil
}

func WriteMeta(pic string, m *Meta) error {
	notes, _, err := Notes(pic)
	if err != nil {
		return err
	}

	data := []byte{}
	if m != nil {
		data, err = json.Marshal(m)
		if err != nil {
			return err
		}
		data = append(data, '\n')
	}

	return ioutil.WriteFile(NotesPath(pic), append(data, []byte(notes)...), 0644)
}

// Checksum returns the sha1 sum of the named pic.  Note that pic can reside
// anywhere and must be a valid path - no searching is done in the library.
func Checksum(pic string) ([]byte, error) {
	f, err := os.Open(pic) // this should not call Filepath
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha1.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func SaveChecksum(pic string) error {
	_, meta, err := Notes(pic)
	if err != nil {
		return err
	} else if meta != nil && len(meta.Sha1) > 0 {
		return nil
	}

	sum, err := Checksum(Filepath(pic))
	if err != nil {
		return err
	}

	meta.Sha1 = fmt.Sprintf("%x", sum)
	return WriteMeta(pic, meta)
}

type NoSumErr string

func (s NoSumErr) Error() string {
	return fmt.Sprintf("%v has no checksum to validate", string(s))
}

func IsNoSum(err error) bool {
	_, ok := err.(NoSumErr)
	return ok
}

func Validate(pic string) error {
	_, meta, err := Notes(pic)
	if err != nil {
		return err
	} else if meta == nil || len(meta.Sha1) == 0 {
		return NoSumErr(pic)
	}

	sum, err := Checksum(pic)
	if err != nil {
		return err
	}

	if meta.Sha1 != fmt.Sprintf("%x", sum) {
		return fmt.Errorf("%v failed validation", pic)
	}

	return nil
}

type DupThumbErr string

func (s DupThumbErr) Error() string {
	return fmt.Sprintf("%v already has a thumbnail", string(s))
}

func IsDupThumb(err error) bool {
	_, ok := err.(DupThumbErr)
	return ok
}

func MakeThumb(pic string, w, h uint) error {
	if _, err := os.Stat(ThumbPath(pic)); err == nil {
		return DupThumbErr(pic)
	}

	f, err := os.Open(Filepath(pic))
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	m := resize.Resize(w, h, img, resize.Bicubic)

	dst, err := os.Create(ThumbPath(pic))
	if err != nil {
		return err
	}
	defer dst.Close()

	return jpeg.Encode(dst, m, nil)
}
