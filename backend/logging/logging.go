package logging

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rwcarlsen/gallery/backend"
)

type Operation string

const (
	OpPut   Operation = "PUT "
	OpGet             = "GET "
	OpDel             = "DEL "
	OpListN           = "LIST"
)

const logFmt = "%v [%v] %v\n"
const timeFmt = "2006/01/02 15:04:05"

// Backend implements github.com/rwcarlsen/gallery/backend.Interface
// wrapping a concrete backend implementation and logs operations performed
// and saves them to the underlying backend.  All operations are forwarded
// unmodified to the wrapped backend.
type Backend struct {
	// The backend to log operations for.
	Back    backend.Interface
	logfile *os.File
}

func New(db backend.Interface, path string) (*Backend, error) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0644)
	if err != nil {
		f, err = os.Create(path)
		if err != nil {
			return nil, err
		}
	}
	f.Seek(0, 2)
	return &Backend{db, f}, nil
}

func (b *Backend) logf(op Operation, msg string) error {
	_, err := b.logfile.WriteString(fmt.Sprintf(logFmt, time.Now().Format(timeFmt), op, msg))
	if err != nil {
		return err
	}
	return err
}

func (lb *Backend) Close() error {
	return lb.logfile.Close()
}

func (b *Backend) Get(key string) (io.ReadCloser, error) {
	rc, err := b.Back.Get(key)
	if err != nil {
		b.logf(OpGet, fmt.Sprintf("%v (ERROR: %v)", key, err))
	} else {
		b.logf(OpGet, key)
	}
	return rc, err
}

func (b *Backend) Put(key string, r io.Reader) (n int64, err error) {
	n, err = b.Back.Put(key, r)
	if err != nil {
		b.logf(OpPut, fmt.Sprintf("%v (ERROR: %v)", key, err))
	} else {
		b.logf(OpPut, fmt.Sprintf("%v (%v bytes)", key, n))
	}
	return n, err
}

func (b *Backend) Del(key string) error {
	err := b.Back.Del(key)
	if err != nil {
		b.logf(OpDel, fmt.Sprintf("%v (ERROR: %v)", key, err))
	} else {
		b.logf(OpDel, fmt.Sprintf("%v", key))
	}
	return err
}

func (b *Backend) Enumerate(prefix string, limit int) ([]string, error) {
	items, err := b.Back.Enumerate(prefix, limit)
	if err != nil {
		b.logf(OpListN, fmt.Sprintf("%v (limit=%v, got=%v, ERROR: %v)", prefix, limit, len(items), err))
	} else {
		b.logf(OpListN, fmt.Sprintf("%v (limit=%v, got=%v)", prefix, limit, len(items)))
	}
	return items, err
}
