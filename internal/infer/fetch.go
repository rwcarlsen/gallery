package infer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var modelFiles = []string{
	ClipImageParam,
	ClipImageBin,
	ClipTextParam,
	ClipTextBin,
	ClipBPEFile,
	SCRFDParam,
	SCRFDBin,
	ArcFaceParam,
	ArcFaceBin,
}

func Fetch(dst, src string, logf func(string, ...any)) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	for _, name := range modelFiles {
		out := filepath.Join(dst, name)
		if fileOK(out) {
			logf("[skip] %s", name)
			continue
		}
		in := filepath.Join(src, name)
		if !fileOK(in) {
			return fmt.Errorf("%s: missing in %s", name, src)
		}
		logf("[copy] %s", name)
		if err := copyFile(in, out); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		logf("[ok] %s", name)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if cerr != nil {
		os.Remove(tmp)
		return cerr
	}
	return os.Rename(tmp, dst)
}
