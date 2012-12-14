
package backend

import (
	"io"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/backend/amz"
)

type Interface interface {
	Name() string
	Exists(path string) bool
	Get(path string) ([]byte, error)
	Put(path string, r io.ReadSeeker) error
	Del(path string) error
	ListN(path string, n int) ([]string, error)
}

func FromCache(name string) Backend {

}

type Type interface {
	Get(params map[string]string) (Backend, error)
}

var types map[Type]//?????

const (
	Amazon Type = "Amazon-S3"
	Local = "Local-HD"
)

func (t Type) Get(params map[string]string) (Backend, error) {
	switch t {
	case Amazon:
		return amzBack(params)
	case Local:
		return localBack(params)
	default:
		return nil, fmt.Errorf("backend: Invalid type %v", s.Type)
	}
}

func localBack() (Backend, error) {
	root, ok := s.Params["Root"]
	if !ok {
		return nil, errors.New("backend: missing 'Root' from Spec")
	}
	name, ok := s.Params["Name"]
	if !ok {
		return nil, errors.New("backend: missing 'Name' from Spec")
	}
	return &localhd.Backend{Root: root, Name: s.Name
}

func amzBack() (Backend, error) {
	root, ok := s.Params[""]
	if !ok {
		return nil, errors.New("backend: missing 'Root' from Spec")
	}
	return &localhd.Backend{Root: root, Name: s.Name
}
