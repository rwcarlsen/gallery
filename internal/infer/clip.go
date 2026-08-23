package infer

import (
	"bytes"
	"fmt"
	"image"

	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/third_party/ncnn"
)

var clipMean = [3]float32{0.48145466, 0.4578275, 0.40821073}
var clipStd = [3]float32{0.26862954, 0.26130258, 0.27577711}

func (e *Engine) EmbedImage(path string) ([]float32, error) {
	img, err := loadRGBA(path)
	if err != nil {
		return nil, err
	}
	return e.embedRGBA(img)
}

func (e *Engine) EmbedBytes(b []byte) ([]float32, error) {
	img, err := decodeRGBA(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return e.embedRGBA(img)
}

func (e *Engine) embedRGBA(img *image.RGBA) ([]float32, error) {
	if e.clipImg == nil {
		return nil, fmt.Errorf("clip image model not loaded")
	}
	crop := centerCropResize(img, 224)
	data := chwNormalize(crop, clipMean, clipStd)
	return runVec(e.clipImg, ncnn.Tensor{Data: data, W: 224, H: 224, C: 3})
}

func (e *Engine) EmbedText(text string) ([]float32, error) {
	if e.clipTxt == nil || e.tok == nil {
		return nil, fmt.Errorf("clip text model not loaded")
	}
	ids, err := e.tok.Encode(text)
	if err != nil {
		return nil, err
	}
	data := make([]float32, len(ids))
	for i, id := range ids {
		data[i] = float32(id)
	}
	ins := map[string]ncnn.Tensor{}
	names := e.clipTxt.InputNames()
	if len(names) == 0 {
		return nil, fmt.Errorf("clip text model has no inputs")
	}
	ins[names[0]] = ncnn.Tensor{Data: data, W: len(data), H: 1, C: 1}
	if len(names) > 1 {
		mask := make([]float32, len(ids))
		for i, id := range ids {
			if id != 0 {
				mask[i] = 1
			}
		}
		mask[0] = 1
		ins[names[1]] = ncnn.Tensor{Data: mask, W: len(mask), H: 1, C: 1}
	}
	outs, err := e.clipTxt.Run(ins)
	if err != nil {
		return nil, err
	}
	return embedFromOuts(outs)
}

func runVec(n *ncnn.Net, t ncnn.Tensor) ([]float32, error) {
	names := n.InputNames()
	if len(names) == 0 {
		return nil, fmt.Errorf("model has no inputs")
	}
	outs, err := n.Run(map[string]ncnn.Tensor{names[0]: t})
	if err != nil {
		return nil, err
	}
	return embedFromOuts(outs)
}

func embedFromOuts(outs map[string]ncnn.Tensor) ([]float32, error) {
	if len(outs) == 0 {
		return nil, fmt.Errorf("no outputs")
	}
	t, ok := outs["out0"]
	if !ok {
		for _, v := range outs {
			t = v
			break
		}
	}
	vec := pickEmbed(t)
	piclib.L2Normalize(vec)
	return vec, nil
}

func pickEmbed(t ncnn.Tensor) []float32 {
	data := t.Data
	if t.H <= 1 && t.C <= 1 {
		return append([]float32(nil), data...)
	}
	dim := t.W
	if dim <= 0 || len(data) < dim {
		return append([]float32(nil), data...)
	}
	return append([]float32(nil), data[len(data)-dim:]...)
}
