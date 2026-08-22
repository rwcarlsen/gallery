package infer

import (
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
)

func loadRGBA(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	if r, ok := img.(*image.RGBA); ok {
		return r, nil
	}
	b := img.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	return dst, nil
}

func centerCropResize(src *image.RGBA, size int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := float64(size) / math.Min(float64(w), float64(h))
	nw := int(math.Round(float64(w) * scale))
	nh := int(math.Round(float64(h) * scale))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	resized := resizeRGBA(src, nw, nh)
	x0 := (nw - size) / 2
	y0 := (nh - size) / 2
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(out, out.Bounds(), resized, image.Pt(x0, y0), draw.Src)
	return out
}

func resizeRGBA(src *image.RGBA, nw, nh int) *image.RGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := float64(y) * float64(sh) / float64(nh)
		y0 := int(sy)
		y1 := y0 + 1
		if y1 >= sh {
			y1 = sh - 1
		}
		fy := sy - float64(y0)
		for x := 0; x < nw; x++ {
			sx := float64(x) * float64(sw) / float64(nw)
			x0 := int(sx)
			x1 := x0 + 1
			if x1 >= sw {
				x1 = sw - 1
			}
			fx := sx - float64(x0)
			c00 := src.RGBAAt(b.Min.X+x0, b.Min.Y+y0)
			c10 := src.RGBAAt(b.Min.X+x1, b.Min.Y+y0)
			c01 := src.RGBAAt(b.Min.X+x0, b.Min.Y+y1)
			c11 := src.RGBAAt(b.Min.X+x1, b.Min.Y+y1)
			dst.SetRGBA(x, y, color.RGBA{
				R: lerpByte(lerpByte(c00.R, c10.R, fx), lerpByte(c01.R, c11.R, fx), fy),
				G: lerpByte(lerpByte(c00.G, c10.G, fx), lerpByte(c01.G, c11.G, fx), fy),
				B: lerpByte(lerpByte(c00.B, c10.B, fx), lerpByte(c01.B, c11.B, fx), fy),
				A: 255,
			})
		}
	}
	return dst
}

func lerpByte(a, b uint8, t float64) uint8 {
	return uint8(float64(a)*(1-t) + float64(b)*t + 0.5)
}

func chwNormalize(img *image.RGBA, mean, std [3]float32) []float32 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]float32, 3*w*h)
	plane := w * h
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			out[i] = (float32(c.R)/255 - mean[0]) / std[0]
			out[plane+i] = (float32(c.G)/255 - mean[1]) / std[1]
			out[2*plane+i] = (float32(c.B)/255 - mean[2]) / std[2]
			i++
		}
	}
	return out
}

func letterbox(src *image.RGBA, size int) (*image.RGBA, float64, int, int) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	scale := math.Min(float64(size)/float64(w), float64(size)/float64(h))
	nw := int(math.Round(float64(w) * scale))
	nh := int(math.Round(float64(h) * scale))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	resized := resizeRGBA(src, nw, nh)
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(out, out.Bounds(), &image.Uniform{C: image.Black}, image.Point{}, draw.Src)
	ox := (size - nw) / 2
	oy := (size - nh) / 2
	draw.Draw(out, image.Rect(ox, oy, ox+nw, oy+nh), resized, image.Point{}, draw.Src)
	return out, scale, ox, oy
}
