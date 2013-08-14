// Package localhd provides a local filesystem based backend/db implementation
// of github.com/rwcarlsen/gallery/backend.Interface
package localhd

import (
	"io"
	"os"
	"path/filepath"
)

// Backend implements github.com/rwcarlsen/gallery/backend.Interface
type Backend struct {
	Root string
}

func (lb *Backend) Get(key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(lb.Root, key)
	return os.Open(fullPath)
}

func (lb *Backend) Close() error { return nil }

func (lb *Backend) Put(key string, r io.Reader) (n int64, err error) {
	fullPath := filepath.Join(lb.Root, key)

	err = os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		return 0, err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return io.Copy(f, r)
}

func (lb *Backend) Del(key string) error {
	fullPath := filepath.Join(lb.Root, key)
	return os.Remove(fullPath)
}

func (lb *Backend) Enumerate(prefix string, limit int) ([]string, error) {
	fullPath := filepath.Join(lb.Root, prefix)
	paths := &[]string{}
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		} else if !info.IsDir() {
			*paths = append(*paths, path)
		}
		return nil
	}

	err := filepath.Walk(fullPath, walkFn)
	if err != nil {
		return nil, err
	}
	return *paths, nil
}
