package infer

import (
	"archive/zip"
	"context"
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
		URL:  "https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_sc.zip",
		Name: SCRFDFile,
		Zip:  "det_500m.onnx",
	},
	{
		URL:  "https://github.com/deepinsight/insightface/releases/download/v0.7/buffalo_sc.zip",
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
				tmp := filepath.Join(dir, ".buffalo_sc.zip")
				logf("[get] %s", f.URL)
				if err := download(f.URL, tmp, logf); err != nil {
					return fmt.Errorf("download buffalo_sc: %w", err)
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
		if err := download(f.URL, dst, logf); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		logf("[ok] %s", f.Name)
	}
	if z, ok := seenZip[defaults[3].URL]; ok {
		_ = os.Remove(z)
	}
	return nil
}

func download(url, dst string, logf func(string, ...any)) error {
	tmp := dst + ".part"
	var last error
	for try := 1; try <= 5; try++ {
		if try > 1 {
			time.Sleep(time.Duration(try-1) * 2 * time.Second)
			logf("[retry] %s (%d/5)", path.Base(url), try)
		}
		if err := fetchOnce(url, dst, tmp, logf); err != nil {
			last = err
			continue
		}
		return nil
	}
	return last
}

func fetchOnce(url, dst, tmp string, logf func(string, ...any)) error {
	var have int64
	if st, err := os.Stat(tmp); err == nil {
		have = st.Size()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "pics-gallery")
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
		logf("[get] resume %d bytes", have)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = 30 * time.Second
	res, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		have = 0
	case http.StatusPartialContent:
	case http.StatusRequestedRangeNotSatisfiable:
		os.Remove(tmp)
		return fmt.Errorf("%s: HTTP %s", url, res.Status)
	default:
		return fmt.Errorf("%s: HTTP %s", url, res.Status)
	}
	flag := os.O_WRONLY | os.O_CREATE
	if have == 0 {
		flag |= os.O_TRUNC
	} else {
		flag |= os.O_APPEND
	}
	f, err := os.OpenFile(tmp, flag, 0644)
	if err != nil {
		return err
	}
	idle := 60 * time.Second
	timer := time.AfterFunc(idle, cancel)
	defer timer.Stop()
	_, err = io.Copy(f, &touchReader{r: res.Body, t: timer, d: idle})
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	return os.Rename(tmp, dst)
}

type touchReader struct {
	r io.Reader
	t *time.Timer
	d time.Duration
}

func (t *touchReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.t.Reset(t.d)
	}
	return n, err
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
