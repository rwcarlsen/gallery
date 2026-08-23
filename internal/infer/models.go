package infer

import (
	"os"
	"path/filepath"
)

const (
	CLIPModelName = "clip-vit-b32"
	FaceModelName = "buffalo-s"

	ClipImageParam = "clip-image.param"
	ClipImageBin   = "clip-image.bin"
	ClipTextParam  = "clip-text.param"
	ClipTextBin    = "clip-text.bin"
	ClipBPEFile    = "clip-bpe.txt.gz"
	SCRFDParam     = "scrfd.param"
	SCRFDBin       = "scrfd.bin"
	ArcFaceParam   = "arcface.param"
	ArcFaceBin     = "arcface.bin"
)

func HasCLIP(dir string) bool {
	return fileOK(filepath.Join(dir, ClipImageParam)) &&
		fileOK(filepath.Join(dir, ClipImageBin)) &&
		fileOK(filepath.Join(dir, ClipTextParam)) &&
		fileOK(filepath.Join(dir, ClipTextBin)) &&
		fileOK(filepath.Join(dir, ClipBPEFile))
}

func HasFaces(dir string) bool {
	return fileOK(filepath.Join(dir, SCRFDParam)) &&
		fileOK(filepath.Join(dir, SCRFDBin)) &&
		fileOK(filepath.Join(dir, ArcFaceParam)) &&
		fileOK(filepath.Join(dir, ArcFaceBin))
}

func fileOK(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() > 0
}
