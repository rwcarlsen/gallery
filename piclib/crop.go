package piclib

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
)

func CropNormJPEG(src image.Image, x, y, w, h float64) ([]byte, error) {
	b := src.Bounds()
	pw, ph := float64(b.Dx()), float64(b.Dy())
	pad := 0.15
	x1 := clampF((x-pad*w)*pw, 0, pw)
	y1 := clampF((y-pad*h)*ph, 0, ph)
	x2 := clampF((x+w+pad*w)*pw, 0, pw)
	y2 := clampF((y+h+pad*h)*ph, 0, ph)
	if x2-x1 < 2 || y2-y1 < 2 {
		x1, y1, x2, y2 = 0, 0, pw, ph
	}
	r := image.Rect(int(x1), int(y1), int(math.Ceil(x2)), int(math.Ceil(y2))).Add(b.Min)
	r = r.Intersect(b)
	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), src, r.Min, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
