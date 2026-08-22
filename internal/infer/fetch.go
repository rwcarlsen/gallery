package infer

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type remoteFile struct {
	URL  string
	Name string
	Zip  string // if set, extract this name from the zip to Name
}

var defaults = []remoteFile{
	{
		URL:  "https://github.com/openai/CLIP/raw/main/clip/bpe_simple_vocab_16e6.txt.gz",
		Name: ClipBPEFile,
	},
	{
		URL:  "https://huggingface.co/Qdrant/clip-ViT-B-32-vision/resolve/main/model.onnx",
		Name: ClipImageFile,
	},
	{
		URL:  "https://huggingface.co/Qdrant/clip-ViT-B-32-text/resolve/main/model.onnx",
		Name: ClipTextFile,
	},
	{
		URL:  "https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_s.zip",
		Name: SCRFDFile,
		Zip:  "det_500m.onnx",
	},
	{
		URL:  "https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_s.zip",
		Name: ArcFaceFile,
		Zip:  "w600k_mbf.onnx",
	},
}

func Fetch(dir string, logf func(string, ...any)) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	seenZip := map[string]string{}
	for _, f := range defaults {
		dst := filepath.Join(dir, f.Name)
		if fileOK(dst) {
			logf("[skip] %s", f.Name)
			continue
		}
		if f.Zip != "" {
			zpath, ok := seenZip[f.URL]
			if !ok {
				tmp := filepath.Join(dir, ".buffalo_s.zip")
				logf("[get] %s", f.URL)
				if err := download(f.URL, tmp); err != nil {
					return fmt.Errorf("download buffalo_s: %w", err)
				}
				seenZip[f.URL] = tmp
				zpath = tmp
			}
			if err := unzipOne(zpath, f.Zip, dst); err != nil {
				return err
			}
			logf("[ok] %s", f.Name)
			continue
		}
		logf("[get] %s", f.URL)
		if err := download(f.URL, dst); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		logf("[ok] %s", f.Name)
	}
	if z, ok := seenZip[defaults[3].URL]; ok {
		_ = os.Remove(z)
	}
	return nil
}

func download(url, dst string) error {
	tmp := dst + ".part"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "pics-gallery")
	client := &http.Client{Timeout: 10 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("%s: HTTP %s", url, res.Status)
	}
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, res.Body)
	cerr := f.Close()
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

func unzipOne(zipPath, want, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	want = path.Base(want)
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if path.Base(f.Name) != want && !strings.HasSuffix(f.Name, "/"+want) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		cerr := out.Close()
		if err != nil {
			os.Remove(dst)
			return err
		}
		return cerr
	}
	return fmt.Errorf("%s not found in %s", want, zipPath)
}
