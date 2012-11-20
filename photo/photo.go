
package photo

import (
	"time"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/gallery/resize"
	"image/jpeg"
	_ "image/gif"
	_ "image/png"
)

type Image []byte

type Exif map[string]interface{}

type Photo struct {
	Orig string
	Thumb1 string
	Thumb2 string
	Uploaded time.Time
	ExifData Exif
	Tags map[string]string
	orig Image
	thumb1 Image
	thumb2 Image
}

func New(name string, data []byte) (*Photo, error) {
	ext := filepath.Ext(name)
	fname := //exif stuff

	r := bytes.NewReader(data)
	orig, _, err := image.Decode(r)
	if err != nil {
		return nil, error
	}

	w, h := 144, 144
	thumb1, err := thumb(orig, w, h)
	if err != nil {
		return nil, error
	}

	w, h := 800, 600
	thumb2, err := thumb(orig, w, h)
	if err != nil {
		return nil, error
	}

	return &Photo{
		Orig: fname + ext,
		Thumb1: fname + "_thumb1" + ext,
		Thumb2: fname + "_thumb2" + ext,
		Uploaded: time.Now(),
		ExifData: x,
		Tags: make(map[string]string),
		orig: data,
		thumb1: thumb1,
		thumb2: thumb2,
	}, nil
}

func (p *Photo) LoadImages(orig, thumb1, thumb2 []byte) {
	p.orig = orig
	p.thumb1 = thumb1
	p.thumb2 = thumb2
}

func (p *Photo) Original() []byte {
	return []byte(p.orig)
}

func (p *Photo) Thumbnail1() []byte {
	return []byte(p.thumb1)
}

func (p *Photo) Thumbnail2() []byte {
	return []byte(thumb1)
}

func thumb(img image.Image, w, h int) (Image, error) {
	bounds := img.Bounds()

	iw, ih := bounds.Dx(), bounds.Dy()
	imgAspect := float32(iw) / float32(ih)
	thumbAspect := float32(w) / float32(h)

	if thumbAspect > imgAspect {
		reduc := ih - float32(ih) / (thumbAspect / imgAspect)
		bounds.Min.Y += int(reduc / 2)
		bounds.Max.Y -= int(reduc / 2)
	} else {
		reduc := iw - float32(iw) / (imgAspect / thumbAspect)
		bounds.Min.X += int(reduc / 2)
		bounds.Max.X -= int(reduc / 2)
	}

	thumb := resize.Resize(orig, bounds, w, h)
	var buf bytes.Buffer
	err := jpeg.Encode(buf, thumb, nil)
	if err != nil {
		return err
	}
	return buf.Bytes()
}

