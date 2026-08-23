package piclib

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func jpegWithExif(taken string, orient uint16) []byte {
	if len(taken) != 19 {
		panic("taken must be YYYY:MM:DD HH:MM:SS")
	}
	var tiff bytes.Buffer
	tiff.Write([]byte{'I', 'I', 0x2a, 0x00})
	binary.Write(&tiff, binary.LittleEndian, uint32(8))
	binary.Write(&tiff, binary.LittleEndian, uint16(2))
	writeIFD(&tiff, 0x0112, 3, 1, uint32(orient))
	writeIFD(&tiff, 0x0132, 2, 20, 38)
	binary.Write(&tiff, binary.LittleEndian, uint32(0))
	tiff.WriteString(taken + "\x00")

	var jpeg bytes.Buffer
	jpeg.Write([]byte{0xff, 0xd8, 0xff, 0xe1})
	binary.Write(&jpeg, binary.BigEndian, uint16(2+6+tiff.Len()))
	jpeg.WriteString("Exif\x00\x00")
	jpeg.Write(tiff.Bytes())
	jpeg.Write([]byte{0xff, 0xd9})
	return jpeg.Bytes()
}

func writeIFD(w *bytes.Buffer, tag, typ uint16, count, val uint32) {
	binary.Write(w, binary.LittleEndian, tag)
	binary.Write(w, binary.LittleEndian, typ)
	binary.Write(w, binary.LittleEndian, count)
	binary.Write(w, binary.LittleEndian, val)
}

func TestReadExif(t *testing.T) {
	tags := ReadExif(bytes.NewReader(jpegWithExif("2019:06:01 12:00:00", 6)))
	got := map[string]string{}
	for _, tg := range tags {
		got[tg.Name] = tg.Value
	}
	if got["DateTime"] != "2019:06:01 12:00:00" {
		t.Fatalf("DateTime=%q tags=%v", got["DateTime"], tags)
	}
	if got["Orientation"] != "6" {
		t.Fatalf("Orientation=%q", got["Orientation"])
	}
	if ReadExif(bytes.NewReader([]byte{0xff, 0xd8, 0xff, 1})) != nil {
		t.Fatal("empty jpeg should have no exif")
	}
}

func TestAddFileExifAndFileMeta(t *testing.T) {
	l := testLib(t)
	src := filepath.Join(t.TempDir(), "shot.jpg")
	if err := os.WriteFile(src, jpegWithExif("2019:06:01 12:00:00", 6), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := l.AddFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if p.Orient != 6 {
		t.Fatalf("orient=%d", p.Orient)
	}
	want := time.Date(2019, 6, 1, 12, 0, 0, 0, time.Local)
	if !p.Taken.Equal(want) {
		t.Fatalf("taken=%v want %v", p.Taken, want)
	}
	if err := p.SetNotes("hi"); err != nil {
		t.Fatal(err)
	}
	tags, err := p.FileMeta()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, tg := range tags {
		got[tg.Name] = tg.Value
	}
	if got["id"] != "1" || got["name"] != "shot.jpg" || got["notes"] != "hi" || got["orient"] != "6" {
		t.Fatalf("%v", got)
	}
	if got["DateTime"] != "2019:06:01 12:00:00" {
		t.Fatalf("DateTime=%q", got["DateTime"])
	}
	if got["path"] != p.Filepath() || got["sum"] == "" {
		t.Fatalf("path/sum %q %q", got["path"], got["sum"])
	}
}
