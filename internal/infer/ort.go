package infer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

const (
	CLIPModelName = "clip-vit-b32"
	FaceModelName = "buffalo-s"

	ClipImageFile = "clip-image.onnx"
	ClipTextFile  = "clip-text.onnx"
	ClipBPEFile   = "clip-bpe.txt.gz"
	SCRFDFile     = "scrfd.onnx"
	ArcFaceFile   = "arcface.onnx"
)

var (
	ortOnce sync.Once
	ortErr  error
)

func initORT(dir string) error {
	ortOnce.Do(func() {
		if p := os.Getenv("ONNXRUNTIME_LIB"); p != "" {
			ort.SetSharedLibraryPath(p)
		} else if p := findORTLib(dir); p != "" {
			ort.SetSharedLibraryPath(p)
		}
		ortErr = ort.InitializeEnvironment()
	})
	return ortErr
}

func findORTLib(dir string) string {
	cands := []string{
		filepath.Join(dir, "libonnxruntime.so"),
		filepath.Join(dir, "libonnxruntime.so.1"),
		"/usr/lib/libonnxruntime.so",
		"/usr/local/lib/libonnxruntime.so",
		"/usr/lib/x86_64-linux-gnu/libonnxruntime.so",
	}
	for _, c := range cands {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

type session struct {
	dyn      *ort.DynamicAdvancedSession
	inNames  []string
	outNames []string
	inTypes  []ort.TensorElementDataType
}

func openSession(path string) (*session, error) {
	ins, outs, err := ort.GetInputOutputInfo(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(ins) == 0 || len(outs) == 0 {
		return nil, fmt.Errorf("%s: missing inputs/outputs", path)
	}
	inNames := make([]string, len(ins))
	inTypes := make([]ort.TensorElementDataType, len(ins))
	outNames := make([]string, len(outs))
	for i, in := range ins {
		inNames[i] = in.Name
		inTypes[i] = in.DataType
	}
	for i, out := range outs {
		outNames[i] = out.Name
	}
	dyn, err := ort.NewDynamicAdvancedSession(path, inNames, outNames, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &session{dyn: dyn, inNames: inNames, outNames: outNames, inTypes: inTypes}, nil
}

func (s *session) close() {
	if s != nil && s.dyn != nil {
		s.dyn.Destroy()
	}
}

func HasCLIP(dir string) bool {
	return fileOK(filepath.Join(dir, ClipImageFile)) &&
		fileOK(filepath.Join(dir, ClipTextFile)) &&
		fileOK(filepath.Join(dir, ClipBPEFile))
}

func HasFaces(dir string) bool {
	return fileOK(filepath.Join(dir, SCRFDFile)) && fileOK(filepath.Join(dir, ArcFaceFile))
}

func fileOK(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() > 0
}
