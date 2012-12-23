
package logging

import (
	"github.com/rwcarlsen/gallery/backend"
)

// Backend implements github.com/rwcarlsen/gallery/backend.Interface
// wrapping a concrete backend implementation and logs operations performed
// and saves them to the underlying database.
type Backend struct {
	Back backend.Interface
	LogPath string
}

func (b *Backend) Name() string {
	return b.Back.Name()
}

func (b *Backend) Exists(path string) bool {
	return b.Back.Exists(path)
}

func (b *Backend) Get(path string) ([]byte, error) {
	return b.Back.Get(path)
}

func (b *Backend) Put(path string, r io.ReadSeeker) error {
	return b.Back.Put(path, r)
}

func (b *Backend) Del(path string) error {
	return b.Back.Del(path)
}

func (b *Backend) ListN(path string, n int) ([]string, error) {
	return b.Back.ListN(path, n)
}

