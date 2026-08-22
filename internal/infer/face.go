package infer

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/rwcarlsen/gallery/piclib"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	detSize   = 640
	detThresh = 0.5
	nmsThresh = 0.4
	recSize   = 112
	minFacePx = 16
)

var arcfaceSrc = [5][2]float64{
	{38.2946, 51.6963},
	{73.5318, 51.5014},
	{56.0252, 71.7366},
	{41.5493, 92.3655},
	{70.7299, 92.2041},
}

type detFace struct {
	score float32
	box   [4]float64 // x1,y1,x2,y2 in letterboxed pixels
	kps   [5][2]float64
}

func (e *Engine) DetectFaces(path string) ([]piclib.Face, error) {
	if e.det == nil || e.rec == nil {
		return nil, fmt.Errorf("face models not loaded")
	}
	img, err := loadRGBA(path)
	if err != nil {
		return nil, err
	}
	lb, scale, ox, oy := letterbox(img, detSize)
	dets, err := e.detect(lb)
	if err != nil {
		return nil, err
	}
	bw := float64(img.Bounds().Dx())
	bh := float64(img.Bounds().Dy())
	var out []piclib.Face
	for _, d := range dets {
		var kps [5][2]float64
		for i := 0; i < 5; i++ {
			kps[i][0] = (d.kps[i][0] - float64(ox)) / scale
			kps[i][1] = (d.kps[i][1] - float64(oy)) / scale
		}
		x1 := clamp((d.box[0]-float64(ox))/scale, 0, bw)
		y1 := clamp((d.box[1]-float64(oy))/scale, 0, bh)
		x2 := clamp((d.box[2]-float64(ox))/scale, 0, bw)
		y2 := clamp((d.box[3]-float64(oy))/scale, 0, bh)
		if x2-x1 < minFacePx || y2-y1 < minFacePx {
			continue
		}
		aligned := warpFace(img, kps)
		vec, err := e.embedFace(aligned)
		if err != nil {
			continue
		}
		out = append(out, piclib.Face{
			X:   x1 / bw,
			Y:   y1 / bh,
			W:   (x2 - x1) / bw,
			H:   (y2 - y1) / bh,
			Vec: vec,
		})
	}
	return out, nil
}

func (e *Engine) detect(img *image.RGBA) ([]detFace, error) {
	data := make([]float32, 3*detSize*detSize)
	plane := detSize * detSize
	i := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			data[i] = (float32(c.R) - 127.5) / 128
			data[plane+i] = (float32(c.G) - 127.5) / 128
			data[2*plane+i] = (float32(c.B) - 127.5) / 128
			i++
		}
	}
	in, err := ort.NewTensor(ort.NewShape(1, 3, detSize, detSize), data)
	if err != nil {
		return nil, err
	}
	defer in.Destroy()
	outs := make([]ort.Value, len(e.det.outNames))
	if err := e.det.dyn.Run([]ort.Value{in}, outs); err != nil {
		return nil, err
	}
	defer func() {
		for _, o := range outs {
			if o != nil {
				o.Destroy()
			}
		}
	}()
	tensors := make([][]float32, len(outs))
	shapes := make([]ort.Shape, len(outs))
	for i, o := range outs {
		t, ok := o.(*ort.Tensor[float32])
		if !ok {
			return nil, fmt.Errorf("scrfd output %d not float32", i)
		}
		tensors[i] = t.GetData()
		shapes[i] = t.GetShape()
	}
	return decodeSCRFD(tensors, shapes, detThresh)
}

func decodeSCRFD(outs [][]float32, shapes []ort.Shape, thr float32) ([]detFace, error) {
	n := len(outs)
	if n == 0 {
		return nil, fmt.Errorf("no scrfd outputs")
	}
	fmc := n / 3
	if fmc < 1 {
		fmc = 1
	}
	strides := []int{8, 16, 32, 64, 128}
	if fmc > len(strides) {
		fmc = len(strides)
	}
	var dets []detFace
	for i := 0; i < fmc && i < len(strides); i++ {
		stride := strides[i]
		scores := outs[i]
		bboxes := outs[i+fmc]
		var kps []float32
		if i+2*fmc < n {
			kps = outs[i+2*fmc]
		}
		h := detSize / stride
		w := detSize / stride
		numAnchors := 2
		expected := h * w * numAnchors
		if len(scores) == 0 {
			continue
		}
		if len(scores) != expected && len(scores)*2 == expected {
			numAnchors = 1
			expected = h * w
		}
		for idx := 0; idx < len(scores) && idx < expected; idx++ {
			if scores[idx] < thr {
				continue
			}
			ay := (idx / numAnchors) / w
			ax := (idx / numAnchors) % w
			cx := float64((ax + 0) * stride)
			cy := float64((ay + 0) * stride)
			// center of cell
			cx = (float64(ax) + 0.5) * float64(stride)
			cy = (float64(ay) + 0.5) * float64(stride)
			off := idx * 4
			if off+3 >= len(bboxes) {
				continue
			}
			l := float64(bboxes[off]) * float64(stride)
			t := float64(bboxes[off+1]) * float64(stride)
			r := float64(bboxes[off+2]) * float64(stride)
			bo := float64(bboxes[off+3]) * float64(stride)
			d := detFace{
				score: scores[idx],
				box:   [4]float64{cx - l, cy - t, cx + r, cy + bo},
			}
			if len(kps) >= (idx+1)*10 {
				base := idx * 10
				for k := 0; k < 5; k++ {
					d.kps[k][0] = cx + float64(kps[base+2*k])*float64(stride)
					d.kps[k][1] = cy + float64(kps[base+2*k+1])*float64(stride)
				}
			}
			dets = append(dets, d)
		}
	}
	return nms(dets, nmsThresh), nil
}

func nms(in []detFace, iouThr float64) []detFace {
	sort.Slice(in, func(i, j int) bool { return in[i].score > in[j].score })
	var keep []detFace
	used := make([]bool, len(in))
	for i := range in {
		if used[i] {
			continue
		}
		keep = append(keep, in[i])
		for j := i + 1; j < len(in); j++ {
			if used[j] {
				continue
			}
			if iou(in[i].box, in[j].box) > iouThr {
				used[j] = true
			}
		}
	}
	return keep
}

func iou(a, b [4]float64) float64 {
	x1 := math.Max(a[0], b[0])
	y1 := math.Max(a[1], b[1])
	x2 := math.Min(a[2], b[2])
	y2 := math.Min(a[3], b[3])
	iw := math.Max(0, x2-x1)
	ih := math.Max(0, y2-y1)
	inter := iw * ih
	aa := math.Max(0, a[2]-a[0]) * math.Max(0, a[3]-a[1])
	bb := math.Max(0, b[2]-b[0]) * math.Max(0, b[3]-b[1])
	u := aa + bb - inter
	if u <= 0 {
		return 0
	}
	return inter / u
}

func (e *Engine) embedFace(img *image.RGBA) ([]float32, error) {
	data := make([]float32, 3*recSize*recSize)
	plane := recSize * recSize
	i := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.RGBAAt(x, y)
			data[i] = (float32(c.R)/255 - 0.5) / 0.5
			data[plane+i] = (float32(c.G)/255 - 0.5) / 0.5
			data[2*plane+i] = (float32(c.B)/255 - 0.5) / 0.5
			i++
		}
	}
	t, err := ort.NewTensor(ort.NewShape(1, 3, recSize, recSize), data)
	if err != nil {
		return nil, err
	}
	return e.runVec(e.rec, []ort.Value{t})
}

func warpFace(src *image.RGBA, kps [5][2]float64) *image.RGBA {
	a, b, tx, ty := similarity(kps, arcfaceSrc)
	det := a*a + b*b
	if det < 1e-12 {
		return resizeRGBA(src, recSize, recSize)
	}
	ia, ib := a/det, -b/det
	// inverse: src = R^{-1} (dst - t)
	out := image.NewRGBA(image.Rect(0, 0, recSize, recSize))
	sb := src.Bounds()
	for y := 0; y < recSize; y++ {
		for x := 0; x < recSize; x++ {
			dx := float64(x) - tx
			dy := float64(y) - ty
			sx := ia*dx - ib*dy
			sy := ib*dx + ia*dy
			out.SetRGBA(x, y, sampleBilinear(src, sb, sx, sy))
		}
	}
	return out
}

func similarity(src, dst [5][2]float64) (a, b, tx, ty float64) {
	// normal equations for [a,b,tx,ty]
	var ata [4][4]float64
	var atb [4]float64
	for i := 0; i < 5; i++ {
		x, y := src[i][0], src[i][1]
		xp, yp := dst[i][0], dst[i][1]
		rows := [2][4]float64{
			{x, -y, 1, 0},
			{y, x, 0, 1},
		}
		rhs := [2]float64{xp, yp}
		for r := 0; r < 2; r++ {
			for c := 0; c < 4; c++ {
				atb[c] += rows[r][c] * rhs[r]
				for k := 0; k < 4; k++ {
					ata[c][k] += rows[r][c] * rows[r][k]
				}
			}
		}
	}
	sol := solve4(ata, atb)
	return sol[0], sol[1], sol[2], sol[3]
}

func solve4(a [4][4]float64, b [4]float64) [4]float64 {
	// gaussian elimination
	m := [4][5]float64{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			m[i][j] = a[i][j]
		}
		m[i][4] = b[i]
	}
	for i := 0; i < 4; i++ {
		piv := i
		for r := i + 1; r < 4; r++ {
			if math.Abs(m[r][i]) > math.Abs(m[piv][i]) {
				piv = r
			}
		}
		m[i], m[piv] = m[piv], m[i]
		if math.Abs(m[i][i]) < 1e-12 {
			continue
		}
		div := m[i][i]
		for c := i; c < 5; c++ {
			m[i][c] /= div
		}
		for r := 0; r < 4; r++ {
			if r == i {
				continue
			}
			f := m[r][i]
			for c := i; c < 5; c++ {
				m[r][c] -= f * m[i][c]
			}
		}
	}
	var x [4]float64
	for i := 0; i < 4; i++ {
		x[i] = m[i][4]
	}
	return x
}

func sampleBilinear(src *image.RGBA, b image.Rectangle, fx, fy float64) color.RGBA {
	if fx < 0 || fy < 0 || fx >= float64(b.Dx()-1) || fy >= float64(b.Dy()-1) {
		return color.RGBA{A: 255}
	}
	x0 := int(fx)
	y0 := int(fy)
	x1 := x0 + 1
	y1 := y0 + 1
	tx := fx - float64(x0)
	ty := fy - float64(y0)
	c00 := src.RGBAAt(b.Min.X+x0, b.Min.Y+y0)
	c10 := src.RGBAAt(b.Min.X+x1, b.Min.Y+y0)
	c01 := src.RGBAAt(b.Min.X+x0, b.Min.Y+y1)
	c11 := src.RGBAAt(b.Min.X+x1, b.Min.Y+y1)
	return color.RGBA{
		R: lerpByte(lerpByte(c00.R, c10.R, tx), lerpByte(c01.R, c11.R, tx), ty),
		G: lerpByte(lerpByte(c00.G, c10.G, tx), lerpByte(c01.G, c11.G, tx), ty),
		B: lerpByte(lerpByte(c00.B, c10.B, tx), lerpByte(c01.B, c11.B, tx), ty),
		A: 255,
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
