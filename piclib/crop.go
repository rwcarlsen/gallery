package piclib

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
)

func OrientNormBox(x, y, w, h float64, orient int) (float64, float64, float64, float64) {
	switch orient {
	case 3, 4:
		x, y = 1-x-w, 1-y-h
	case 5, 6:
		x, y, w, h = 1-y-h, x, h, w
	case 7, 8:
		x, y, w, h = y, 1-x-w, h, w
	}
	switch orient {
	case 2, 5, 4, 7:
		x = 1 - x - w
	}
	return x, y, w, h
}

func CropNormJPEG(src image.Image, x, y, w, h float64) ([]byte, error) {
	b := src.Bounds()
	pw, ph := float64(b.Dx()), float64(b.Dy())
	pad := 0.15
	cx := (x + w/2) * pw
	cy := (y + h/2) * ph
	side := math.Max(w*pw, h*ph) * (1 + 2*pad)
	if maxSide := math.Min(pw, ph); side > maxSide {
		side = maxSide
	}
	x1, y1 := cx-side/2, cy-side/2
	if pw > side {
		x1 = clampF(x1, 0, pw-side)
	} else {
		x1 = 0
	}
	if ph > side {
		y1 = clampF(y1, 0, ph-side)
	} else {
		y1 = 0
	}
	x2, y2 := x1+side, y1+side
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
