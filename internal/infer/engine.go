package infer

import (
	"fmt"
	"path/filepath"

	"github.com/rwcarlsen/gallery/third_party/ncnn"
)

type Engine struct {
	dir     string
	tok     *Tokenizer
	clipImg *ncnn.Net
	clipTxt *ncnn.Net
	det     *ncnn.Net
	rec     *ncnn.Net
}

func Open(dir string, wantClip, wantFaces bool) (*Engine, error) {
	if wantClip && !HasCLIP(dir) {
		return nil, fmt.Errorf("clip models missing in %s (run pics index -fetch)", dir)
	}
	if wantFaces && !HasFaces(dir) {
		return nil, fmt.Errorf("face models missing in %s (run pics index -fetch)", dir)
	}
	e := &Engine{dir: dir}
	var err error
	if wantClip {
		e.tok, err = LoadTokenizer(filepath.Join(dir, ClipBPEFile))
		if err != nil {
			return nil, fmt.Errorf("clip tokenizer: %w", err)
		}
		e.clipImg, err = ncnn.Open(filepath.Join(dir, ClipImageParam), filepath.Join(dir, ClipImageBin))
		if err != nil {
			e.Close()
			return nil, err
		}
		e.clipTxt, err = ncnn.Open(filepath.Join(dir, ClipTextParam), filepath.Join(dir, ClipTextBin))
		if err != nil {
			e.Close()
			return nil, err
		}
	}
	if wantFaces {
		e.det, err = ncnn.Open(filepath.Join(dir, SCRFDParam), filepath.Join(dir, SCRFDBin))
		if err != nil {
			e.Close()
			return nil, err
		}
		e.rec, err = ncnn.Open(filepath.Join(dir, ArcFaceParam), filepath.Join(dir, ArcFaceBin))
		if err != nil {
			e.Close()
			return nil, err
		}
	}
	return e, nil
}

func (e *Engine) Close() {
	if e == nil {
		return
	}
	e.clipImg.Close()
	e.clipTxt.Close()
	e.det.Close()
	e.rec.Close()
}

func (e *Engine) CanCLIP() bool  { return e != nil && e.clipImg != nil && e.clipTxt != nil }
func (e *Engine) CanFaces() bool { return e != nil && e.det != nil && e.rec != nil }
