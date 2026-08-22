package infer

import (
	"fmt"
	"path/filepath"
)

type Engine struct {
	dir     string
	tok     *Tokenizer
	clipImg *session
	clipTxt *session
	det     *session
	rec     *session
}

func Open(dir string, wantClip, wantFaces bool) (*Engine, error) {
	if wantClip && !HasCLIP(dir) {
		return nil, fmt.Errorf("clip models missing in %s (run pics index -fetch)", dir)
	}
	if wantFaces && !HasFaces(dir) {
		return nil, fmt.Errorf("face models missing in %s (run pics index -fetch)", dir)
	}
	if err := initORT(dir); err != nil {
		return nil, fmt.Errorf("onnxruntime: %w (set ONNXRUNTIME_LIB or put libonnxruntime.so in %s)", err, dir)
	}
	e := &Engine{dir: dir}
	var err error
	if wantClip {
		e.tok, err = LoadTokenizer(filepath.Join(dir, ClipBPEFile))
		if err != nil {
			return nil, fmt.Errorf("clip tokenizer: %w", err)
		}
		e.clipImg, err = openSession(filepath.Join(dir, ClipImageFile))
		if err != nil {
			e.Close()
			return nil, err
		}
		e.clipTxt, err = openSession(filepath.Join(dir, ClipTextFile))
		if err != nil {
			e.Close()
			return nil, err
		}
	}
	if wantFaces {
		if !HasFaces(dir) {
			e.Close()
			return nil, fmt.Errorf("face models missing in %s (run pics index -fetch)", dir)
		}
		e.det, err = openSession(filepath.Join(dir, SCRFDFile))
		if err != nil {
			e.Close()
			return nil, err
		}
		e.rec, err = openSession(filepath.Join(dir, ArcFaceFile))
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
	e.clipImg.close()
	e.clipTxt.close()
	e.det.close()
	e.rec.close()
}

func (e *Engine) CanCLIP() bool  { return e != nil && e.clipImg != nil && e.clipTxt != nil }
func (e *Engine) CanFaces() bool { return e != nil && e.det != nil && e.rec != nil }
