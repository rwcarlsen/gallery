package infer

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

const modelRelease = "https://github.com/rwcarlsen/gallery/releases/download/models-ncnn-1"

type remoteFile struct {
	URL  string
	Name string
}

var defaults = []remoteFile{
	{
		URL:  "https://github.com/openai/CLIP/raw/main/clip/bpe_simple_vocab_16e6.txt.gz",
		Name: ClipBPEFile,
	},
	{URL: modelRelease + "/" + ClipImageParam, Name: ClipImageParam},
	{URL: modelRelease + "/" + ClipImageBin, Name: ClipImageBin},
	{URL: modelRelease + "/" + ClipTextParam, Name: ClipTextParam},
	{URL: modelRelease + "/" + ClipTextBin, Name: ClipTextBin},
	{URL: modelRelease + "/" + SCRFDParam, Name: SCRFDParam},
	{URL: modelRelease + "/" + SCRFDBin, Name: SCRFDBin},
	{URL: modelRelease + "/" + ArcFaceParam, Name: ArcFaceParam},
	{URL: modelRelease + "/" + ArcFaceBin, Name: ArcFaceBin},
}

func Fetch(dir string, logf func(string, ...any)) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	for _, f := range defaults {
		dst := filepath.Join(dir, f.Name)
		if fileOK(dst) {
			logf("[skip] %s", f.Name)
			continue
		}
		logf("[get] %s", f.URL)
		if err := download(f.URL, dst, logf); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		logf("[ok] %s", f.Name)
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
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
	_, err = io.Copy(f, res.Body)
	cerr := f.Close()
	if err != nil {
		return err
	}
	if cerr != nil {
		return cerr
	}
	return os.Rename(tmp, dst)
}
