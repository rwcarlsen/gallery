package piclib

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/mknote"
	"github.com/rwcarlsen/goexif/tiff"
)

func init() {
	exif.RegisterParsers(mknote.All...)
}

type MetaTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (p *Pic) Catalog() []MetaTag {
	tags := []MetaTag{
		{"id", strconv.Itoa(p.Id)},
		{"name", p.Name},
		{"sum", hex.EncodeToString(p.Sum)},
		{"path", p.Filepath()},
		{"added", p.Added.Format(time.RFC3339)},
		{"taken", p.Taken.Format(time.RFC3339)},
		{"orient", strconv.Itoa(p.Orient)},
	}
	if notes, err := p.GetNotes(); err == nil && notes != "" {
		tags = append(tags, MetaTag{"notes", notes})
	}
	return tags
}

func (p *Pic) Exif() ([]MetaTag, error) {
	f, err := os.Open(p.Filepath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ReadExif(f), nil
}

func (p *Pic) FileMeta() ([]MetaTag, error) {
	tags := p.Catalog()
	ex, err := p.Exif()
	if err != nil {
		return tags, err
	}
	return append(tags, ex...), nil
}

func ReadExif(r io.Reader) []MetaTag {
	x, err := exif.Decode(r)
	if x == nil || (err != nil && exif.IsCriticalError(err)) {
		return nil
	}
	var tags []MetaTag
	_ = x.Walk(metaWalker{func(name exif.FieldName, tag *tiff.Tag) error {
		if skipExif(name, tag) {
			return nil
		}
		tags = append(tags, MetaTag{string(name), formatExif(tag)})
		return nil
	}})
	if lat, lon, err := x.LatLong(); err == nil {
		tags = append(tags, MetaTag{"GPS", fmt.Sprintf("%f, %f", lat, lon)})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
	return tags
}

type metaWalker struct {
	fn func(exif.FieldName, *tiff.Tag) error
}

func (w metaWalker) Walk(name exif.FieldName, tag *tiff.Tag) error {
	return w.fn(name, tag)
}

func skipExif(name exif.FieldName, tag *tiff.Tag) bool {
	switch name {
	case exif.MakerNote, exif.ThumbJPEGInterchangeFormat, exif.ThumbJPEGInterchangeFormatLength:
		return true
	}
	return tag.Count > 64 && tag.Format() != tiff.StringVal
}

func formatExif(tag *tiff.Tag) string {
	if s, err := tag.StringVal(); err == nil {
		return strings.TrimRight(s, "\x00")
	}
	return strings.Trim(tag.String(), `"`)
}
