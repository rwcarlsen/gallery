// Package backend contains a standard interface and tools for working with
// simple databases.
package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/goamz/aws"
)

// Interface specifies the methods that each backend database
// implementation must satisfy.
type Interface interface {
	// Get returns the file/object at path.
	Get(key string) (io.ReadCloser, error)
	// Put writes all data in r to path - overwriting any existing
	// file/object.
	Put(key string, r io.Reader) (n int64, err error)
	// Del removes the file/object at path.
	Del(key string) error
	// ListN returns a list of n full, absolute paths for every file/object
	// recursively under path.
	Enumerate(prefix string, limit int) ([]string, error)
	Close() error
}

func Exists(db Interface, key string) bool {
	rc, err := db.Get(key)
	if err == nil {
		rc.Close()
		return true
	}
	return false
}

func GetBytes(db Interface, key string) ([]byte, error) {
	rc, err := db.Get(key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return ioutil.ReadAll(rc)
}

// Params is used to hold/specify details required for the initialization
// of standard and custom backends.
type Params map[string]string

// TypeFunc is an abstraction allowing package-external backends to be
// handled by this package. A TypeFunc instance should return a
// ready-to-use backend initialized with Params.
type TypeFunc func(Params) (Interface, error)

// Type specifies a unique kind of backend. There is a one-to-one
// correspondence between backend Types and TypeFunc's.
type Type string

const (
	Amazon Type = "Amazon-S3"
	Local       = "Local-HD"
	dummy       = "dummy" // used for testing
)

var types = map[Type]TypeFunc{}

func init() {
	Register(Amazon, amzBack)
	Register(Local, localBack)
	Register(dummy, dummyBack)
}

// Register enables backends of type t to be created by Make functions and
// methods in this package.
func Register(t Type, fn TypeFunc) {
	types[t] = fn
}

// Make creates a backend of type t, initialized with the given params.
// params must contain required information for the specified backend type.
// An error is returned if t is an unregistered type or if params do not
// contain all pertinent information to initialize a backend of type t.
func Make(t Type, params Params) (Interface, error) {
	if fn, ok := types[t]; ok {
		return fn(params)
	}
	return nil, fmt.Errorf("backend: Invalid type %v", t)
}

func localBack(params Params) (Interface, error) {
	root, ok := params["Root"]
	if !ok {
		return nil, errors.New("backend: missing 'Root' from Params")
	}
	return &localhd.Backend{Root: root}, nil
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

	auth := aws.Auth{AccessKey: keyid, SecretKey: key}
	db := amz.New(auth, aws.USEast)
	return db, nil
}

func dummyBack(params Params) (Interface, error) {
	return nil, nil
}

// Spec is a convenient way to group a specific set of config Params for a
// backend together with its corresponding Type.
type Spec struct {
	Btype   Type
	Bparams Params
}

// Make creates a backend from the Spec. This is a shortcut for the Make
// function.
func (s *Spec) Make() (Interface, error) {
	return Make(s.Btype, s.Bparams)
}

// LoadSpec creates a spec-configured backend by decoding JSON data from r.
func LoadSpec(r io.Reader) (Interface, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	s := &Spec{}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, prettySyntaxError(string(data), err)
	}

	return s.Make()
}

// Save writes the Spec in JSON format to w.
func (s *Spec) Save(w io.Writer) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

func prettySyntaxError(js string, err error) error {
	syntax, ok := err.(*json.SyntaxError)
	if !ok {
		return err
	}

	start, end := strings.LastIndex(js[:syntax.Offset],
		"\n")+1, len(js)
	if idx := strings.Index(js[start:], "\n"); idx >= 0 {
		end = start + idx
	}

	line, pos := strings.Count(js[:start], "\n"), int(syntax.Offset)-start-1

	msg := fmt.Sprintf("Error in line %d: %s\n", line+1, err)
	msg += fmt.Sprintf("%s\n%s^", js[start:end], strings.Repeat("", pos))
	return pretty(msg)
}

type pretty string

func (p pretty) Error() string {
	return string(p)
}
