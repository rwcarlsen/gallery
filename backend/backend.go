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
	// Name returns the name of the backend (not Type).
	Name() string
	// Exists returns whether or not a file/object exists at path.
	Exists(path string) bool
	// Get returns the file/object at path.
	Get(path string) ([]byte, error)
	// Put writes all data in r to path - overwriting any existing
	// file/object.
	Put(path string, r io.ReadSeeker) error
	// Del removes the file/object at path.
	Del(path string) error
	// ListN returns a list of n full, absolute paths for every file/object
	// recursively under path.
	ListN(path string, n int) ([]string, error)
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

// SpecList is a convenient way to manage multiple backend Spec's together
// as a group (e.g. saving to/from a config file, etc).
type SpecList struct {
	list map[string]*Spec
}

// LoadSpecList creates a SpecList by decoding JSON data from r.
func LoadSpecList(r io.Reader) (*SpecList, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	list := map[string]*Spec{}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, prettySyntaxError(string(data), err)
	}

	return &SpecList{list: list}, nil
}

// Save writes the SpecList in JSON format to w.
func (s *SpecList) Save(w io.Writer) error {
	data, err := json.Marshal(s.list)
	if err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

// Get retrieves the named Spec. It returns nil if name is not found.
func (s *SpecList) Get(name string) *Spec {
	s.init()
	return s.list[name]
}

// Set adds a new Spec with the given name to the specset. If name is
// already in the specset, it is overwritten.
func (s *SpecList) Set(name string, spec *Spec) {
	s.init()
	s.list[name] = spec
}

// Make creates a backend from Spec identified by name. This is a shortcut
// for ".Get(...).Make(...)".
func (s *SpecList) Make(name string) (Interface, error) {
	s.init()
	if spec, ok := s.list[name]; ok {
		return spec.Make()
	}
	return nil, fmt.Errorf("backend: name '%v' not found in SpecList", name)
}

func (s *SpecList) init() {
	if s.list == nil {
		s.list = make(map[string]*Spec)
	}
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

