
package logging

import (
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
// and saves them to the underlying database.
type Backend struct {
	Back backend.Interface
	Stream io.Writer
}

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
		b.logf(OpGet, fmt.Sprintf("%v (%v)", path, err.Error()))
	}

	return data, err
}

// TODO: needs work...?
func (b *Backend) Put(path string, r io.ReadSeeker) error {
	h := sha1.New()
	n, err := io.Copy(h, r)
	if err := r.Seek(0, 0); err != nil {
		b.logf(OpPut, fmt.Sprintf("%v (%v)", path, err.Error()))
		return err
	}

	if err == nil {
		sum := h.Sum(nil)
		msg := fmt.Sprintf("%v (%v bytes, sha1:%x)", path, n, sum)
		b.logf(OpPut, msg)
	} else {
		msg := fmt.Sprintf("%v (sha1 hash failed)", path)
		b.logf(OpPut, msg)
	}

	if err = b.Back.Put(path, r); err != nil {
		
	}
	return err
}

func (b *Backend) Del(path string) error {
	return b.Back.Del(path)
}

func (b *Backend) ListN(path string, n int) ([]string, error) {
	return b.Back.ListN(path, n)
}

