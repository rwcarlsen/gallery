package piclib

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"math"
	"testing"

	"github.com/disintegration/imaging"
)

func TestOrientNormBox(t *testing.T) {
	x, y, w, h := 0.1, 0.2, 0.3, 0.4
	want := map[int][4]float64{
		0: {0.1, 0.2, 0.3, 0.4},
		1: {0.1, 0.2, 0.3, 0.4},
		2: {0.6, 0.2, 0.3, 0.4},
		3: {0.6, 0.4, 0.3, 0.4},
		4: {0.1, 0.4, 0.3, 0.4},
		5: {0.2, 0.1, 0.4, 0.3},
		6: {0.4, 0.1, 0.4, 0.3},
		7: {0.4, 0.6, 0.4, 0.3},
		8: {0.2, 0.6, 0.4, 0.3},
	}
	for orient, exp := range want {
		gx, gy, gw, gh := OrientNormBox(x, y, w, h, orient)
		if !near(gx, exp[0]) || !near(gy, exp[1]) || !near(gw, exp[2]) || !near(gh, exp[3]) {
			t.Fatalf("orient %d: got %v want %v", orient, [4]float64{gx, gy, gw, gh}, exp)
		}
	}
}

func TestCropNormJPEGSquare(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(src, src.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	face := image.Rect(40, 40, 50, 60)
	draw.Draw(src, face, &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	jpeg, err := CropNormJPEG(src, 0.4, 0.4, 0.1, 0.2)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(jpeg))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() {
		t.Fatalf("not square: %dx%d", b.Dx(), b.Dy())
	}
	if b.Dx() < 20 || b.Dx() > 30 {
		t.Fatalf("side %d want ~26", b.Dx())
	}
}

func TestCropNormJPEGEdge(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 80))
	draw.Draw(src, src.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	jpeg, err := CropNormJPEG(src, 0, 0, 0.1, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(jpeg))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != b.Dy() {
		t.Fatalf("not square: %dx%d", b.Dx(), b.Dy())
	}
}

func TestOrientNormBoxMatchesMakeThumb(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 20, 10))
	draw.Draw(src, src.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(src, image.Rect(2, 2, 6, 6), &image.Uniform{C: color.Black}, image.Point{}, draw.Src)
	const fx, fy, fw, fh = 2.0 / 20, 2.0 / 10, 4.0 / 20, 4.0 / 10
	for orient := 0; orient <= 8; orient++ {
		m := image.Image(src)
		switch orient {
		case 3, 4:
			m = imaging.Rotate180(m)
		case 5, 6:
			m = imaging.Rotate270(m)
		case 7, 8:
			m = imaging.Rotate90(m)
		}
		switch orient {
		case 2, 5, 4, 7:
			m = imaging.FlipH(m)
		}
		x, y, w, h := OrientNormBox(fx, fy, fw, fh, orient)
		b := m.Bounds()
		cx := b.Min.X + int((x+w/2)*float64(b.Dx()))
		cy := b.Min.Y + int((y+h/2)*float64(b.Dy()))
		r, g, bl, _ := m.At(cx, cy).RGBA()
		if r != 0 || g != 0 || bl != 0 {
			t.Fatalf("orient %d: center (%d,%d) not black", orient, cx, cy)
		}
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
