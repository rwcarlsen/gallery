
package localhd

import (
	"os"
	"errors"
	"io/ioutil"
	"path/filepath"
)

type Backend struct {
	Root string
}

func (lb *Backend) Put(path, name string, data []byte) error {
	fullPath := filepath.Join(lb.Root, path, name)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}

	n, err := f.Write(data)
	if n < len(data) {
		return errors.New("local: failed to write entire data stream")
	} else if err != nil {
		return err
	}

	return nil
}

func (lb *Backend) Exists(path, name string) bool {
	fullPath := filepath.Join(lb.Root, path, name)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (lb *Backend) ListN(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	N := n
	if n == 0 {
		N = -1
	}

	names, err := f.Readdirnames(N)
	if err != nil {
		return nil, err
	}

	return names, nil
}

func (lb *Backend) Get(path, name string) ([]byte, error) {
	fullPath := filepath.Join(lb.Root, path, name)

	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	return data, nil
}
