package infer

import (
	"fmt"

	"github.com/rwcarlsen/gallery/piclib"
	ort "github.com/yalue/onnxruntime_go"
)

var clipMean = [3]float32{0.48145466, 0.4578275, 0.40821073}
var clipStd = [3]float32{0.26862954, 0.26130258, 0.27577711}

func (e *Engine) EmbedImage(path string) ([]float32, error) {
	if e.clipImg == nil {
		return nil, fmt.Errorf("clip image model not loaded")
	}
	img, err := loadRGBA(path)
	if err != nil {
		return nil, err
	}
	crop := centerCropResize(img, 224)
	data := chwNormalize(crop, clipMean, clipStd)
	t, err := ort.NewTensor(ort.NewShape(1, 3, 224, 224), data)
	if err != nil {
		return nil, err
	}
	return e.runVec(e.clipImg, []ort.Value{t})
}

func (e *Engine) EmbedText(text string) ([]float32, error) {
	if e.clipTxt == nil || e.tok == nil {
		return nil, fmt.Errorf("clip text model not loaded")
	}
	ids, err := e.tok.Encode(text)
	if err != nil {
		return nil, err
	}
	idTen, err := intTensor(e.clipTxt, 0, ids)
	if err != nil {
		return nil, err
	}
	defer idTen.Destroy()
	ins := []ort.Value{idTen}
	if len(e.clipTxt.inNames) > 1 {
		mask := make([]int64, ctxLen)
		for i, id := range ids {
			if id != 0 {
				mask[i] = 1
			}
		}
		mask[0] = 1
		mt, err := intTensor(e.clipTxt, 1, mask)
		if err != nil {
			return nil, err
		}
		defer mt.Destroy()
		ins = append(ins, mt)
	}
	return e.runInputs(e.clipTxt, ins)
}

func intTensor(s *session, idx int, ids []int64) (ort.Value, error) {
	if idx < len(s.inTypes) && s.inTypes[idx] == ort.TensorElementDataTypeInt32 {
		v := make([]int32, len(ids))
		for i, x := range ids {
			v[i] = int32(x)
		}
		return ort.NewTensor(ort.NewShape(1, int64(len(v))), v)
	}
	return ort.NewTensor(ort.NewShape(1, int64(len(ids))), ids)
}

func (e *Engine) runVec(s *session, ins []ort.Value) ([]float32, error) {
	defer func() {
		for _, in := range ins {
			if in != nil {
				in.Destroy()
			}
		}
	}()
	return e.runInputs(s, ins)
}

func (e *Engine) runInputs(s *session, ins []ort.Value) ([]float32, error) {
	outs := make([]ort.Value, len(s.outNames))
	if err := s.dyn.Run(ins, outs); err != nil {
		return nil, err
	}
	defer func() {
		for _, o := range outs {
			if o != nil {
				o.Destroy()
			}
		}
	}()
	t, ok := outs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output type %T", outs[0])
	}
	data := append([]float32(nil), t.GetData()...)
	sh := t.GetShape()
	vec := pickEmbed(data, sh)
	piclib.L2Normalize(vec)
	return vec, nil
}

func pickEmbed(data []float32, shape ort.Shape) []float32 {
	if len(shape) <= 2 {
		return data
	}
	// [1, seq, dim] — take last row (EOT-ish)
	dim := int(shape[len(shape)-1])
	if dim <= 0 || len(data) < dim {
		return data
	}
	off := len(data) - dim
	return data[off:]
}
