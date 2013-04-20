package logging

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"time"

	"github.com/rwcarlsen/gallery/backend"
)

type Operation string

const (
	OpPut    Operation = "PUT"
	OpGet              = "GET"
	OpName             = "NAME"
	OpExists           = "EXISTS"
	OpDel              = "DEL"
	OpListN            = "LIST"
)

const logFmt = "[%v] %v %v\n"
const DefaultPath = ".dblog"

// Backend implements github.com/rwcarlsen/gallery/backend.Interface
// wrapping a concrete backend implementation and logs operations performed
// and saves them to the underlying backend.  All operations are forwarded
// unmodified to the wrapped backend.
type Backend struct {
	// The backend to log operations for.
	Back backend.Interface
	// Path specifies the path+filename for the logfile
	Path   string
	cached []byte
}

func (b *Backend) logf(op Operation, msg string) error {
	if b.Path == "" {
		b.Path = DefaultPath
	}
	if b.cached == nil {
		b.cached, _ = b.Back.Get(b.Path)
	}
	b.cached = append(b.cached, []byte(fmt.Sprintf(logFmt, time.Now(), op, msg))...)
	return b.Back.Put(b.Path, bytes.NewReader(b.cached))
}

func (b *Backend) Name() string {
	name := b.Back.Name()
	b.logf(OpName, name)
	return name
}

func (b *Backend) Exists(path string) bool {
	exists := b.Back.Exists(path)
	b.logf(OpExists, fmt.Sprintf("%v (%v)", path, exists))
	return exists
}

func (b *Backend) Get(path string) ([]byte, error) {
	data, err := b.Back.Get(path)

	if err != nil {
		h := sha1.New()
		h.Write(data)
		sum := h.Sum(nil)
		b.logf(OpGet, fmt.Sprintf("%v (%v bytes, sha1:%x)", path, len(data), sum))
	} else {
		b.logf(OpGet, fmt.Sprintf("%v (ERROR: %v)", path, err.Error()))
	}

	return data, err
}

func (b *Backend) Put(path string, r io.ReadSeeker) error {
	if err := b.Back.Put(path, r); err != nil {
		b.logf(OpPut, fmt.Sprintf("%v (ERROR: %v)", path, err.Error()))
		return err
	}

	h := sha1.New()
	if _, err := r.Seek(0, 0); err != nil {
		b.logf(OpPut, fmt.Sprintf("%v (? bytes, sha1:?)", path))
	} else if n, err := io.Copy(h, r); err != nil {
		b.logf(OpPut, fmt.Sprintf("%v (? bytes, sha1:?)", path))
	} else {
		b.logf(OpPut, fmt.Sprintf("%v (%v bytes, sha1:%v)", path, n, h.Sum(nil)))
	}
	return nil
}

func (b *Backend) Del(path string) error {
	err := b.Back.Del(path)
	if err != nil {
		b.logf(OpDel, fmt.Sprintf("%v (ERROR: %v)", path, err.Error()))
	} else {
		b.logf(OpDel, fmt.Sprintf("%v", path))
	}
	return err
}

func (b *Backend) ListN(path string, n int) ([]string, error) {
	items, err := b.Back.ListN(path, n)
	if err != nil {
		b.logf(OpListN, fmt.Sprintf("%v (ask=%v, got=%v, ERROR: %v)", path, n, len(items), err.Error()))
	} else {
		b.logf(OpListN, fmt.Sprintf("%v (ask=%v, got=%v)", path, n, len(items)))
	}
	return items, err
}
