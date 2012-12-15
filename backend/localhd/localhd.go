
// Package localhd provides a local filesystem based backend/db implementation
// of github.com/rwcarlsen/gallery/backend.Interface
package localhd

import (
	"io"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Backend implements github.com/rwcarlsen/gallery/backend.Interface
type Backend struct {
	Root   string
	DbName string
}

func (lb *Backend) Name() string {
	return lb.DbName
}

func (lb *Backend) Exists(path string) bool {
	fullPath := filepath.Join(lb.Root, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (lb *Backend) Get(path string) ([]byte, error) {
	fullPath := filepath.Join(lb.Root, path)

	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (lb *Backend) Put(path string, r io.ReadSeeker) error {
	fullPath := filepath.Join(lb.Root, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		return err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return err
	}

	return nil
}

func (lb *Backend) Del(path string) error {
	fullPath := filepath.Join(lb.Root, path)
	return os.Remove(fullPath)
}

func (lb *Backend) ListN(path string, n int) ([]string, error) {
	fullPath := filepath.Join(lb.Root, path)

	ch := make(chan string)
	done := make(chan bool)
	errCh := make(chan error, 2)
	go func() {
		errCh <- filepath.Walk(fullPath, getWalker(ch, done, lb.Root))
		close(ch)
		close(errCh)
	}()

	var names []string
	count := 0
	for name := range ch {
		if n > 0 && count == n {
			done <- true
			break
		}
		names = append(names, name)
		count++
	}

	if err := <-errCh; err != nil && err.Error() != "incomplete" {
		return nil, err
	}

	return names, nil
}

func getWalker(ch chan string, done chan bool, base string) func(string, os.FileInfo, error) error {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(base, path)
			select {
			case ch <- rel:
			case <-done:
				return errors.New("incomplete")
			}
		}
		return nil
	}
}

