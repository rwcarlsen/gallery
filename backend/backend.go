
package backend

import (
	"io"
	"fmt"
	"errors"

	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/goamz/aws"
)

type Interface interface {
	Name() string
	Exists(path string) bool
	Get(path string) ([]byte, error)
	Put(path string, r io.ReadSeeker) error
	Del(path string) error
	ListN(path string, n int) ([]string, error)
}

type Params map[string]string

type TypeFunc func(Params) (Interface, error)

var types = map[Type]TypeFunc{}

type Type string

const (
	Amazon Type = "Amazon-S3"
	Local = "Local-HD"
)

func init() {
	types[Amazon] = amzBack
	types[Local] = localBack
}

func Register(t Type, fn TypeFunc) {
	types[t] = fn
}

func localBack(params Params) (Interface, error) {
	root, ok := params["Root"]
	if !ok {
		return nil, errors.New("backend: missing 'Root' from Params")
	}
	name, ok := params["Name"]
	if !ok {
		return nil, errors.New("backend: missing 'Name' from Params")
	}
	return &localhd.Backend{Root: root, DbName: name}, nil
}

func amzBack(params Params) (Interface, error) {
	keyid, ok := params["AccessKeyId"]
	if !ok {
		return nil, errors.New("backend: missing 'AccessKeyId' from Params")
	}
	key, ok := params["SecretAccessKey"]
	if !ok {
		return nil, errors.New("backend: missing 'SecretAccessKey' from Params")
	}
	name, ok := params["Name"]
	if !ok {
		return nil, errors.New("backend: missing 'Name' from Params")
	}

	auth := aws.Auth{AccessKey: keyid, SecretKey: key}
	db := amz.New(auth, aws.USEast)
	db.DbName = name
	return db, nil
}

type Spec struct {
	Btype Type
	Bparams Params
}

func (s *Spec) Make() (Interface, error) {
	if fn, ok := types[s.Btype]; ok {
		return fn(s.Bparams)
	}
	return nil, fmt.Errorf("backend: Invalid type %v", s.Btype)
}

