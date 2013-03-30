
package logging

import (
	"fmt"
	"time"
	"io"
	"crypto/sha1"

	"github.com/rwcarlsen/gallery/backend"
)

type Operation string

const (
	OpPut Operation = "PUT"
	OpGet = "GET"
	OpName = "NAME"
	OpExists = "EXISTS"
	OpDel = "DEL"
	OpListN = "LIST"
)

const logFmt = "[%v] %v %v"

// Backend implements github.com/rwcarlsen/gallery/backend.Interface
// wrapping a concrete backend implementation and logs operations performed
// and saves them to the underlying database.  All operations are forwarded
// unmodified to the wrapped backend.
type Backend struct {
	Back backend.Interface
	Stream io.Writer
}

// New creates and returns a new logging backend wrapping b writing its
// activity log to w.
func New(b backend.Interface, w io.Writer) backend.Interface {
	return &Backend{
		Back: b,
		Stream: w,
	}
}

func (b *Backend) logf(op Operation, msg string) {
	fmt.Fprintf(b.Stream, logFmt, time.Now(), op, msg)
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

