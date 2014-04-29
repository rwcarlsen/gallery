// Package piclib provides tools backend-agnostic management of large photo collections.
package piclib

import (
	"fmt"
	_ "image/gif"
	_ "image/png"
	"path/filepath"
)

const (
	NoDate   = "-NoDate"
	NameSep  = "-sep-"
	NotesExt = ".notes"
)

const (
	nameTimeFmt = "2006-01-02-15-04-05"
	Version     = "0.1"
)

type DupErr string

func (e DupErr) Error() string {
	b := filepath.Base(string(e))
	return fmt.Sprintf("'%s' already exists in library", b)
}

// Library manages and organizes collections of Photos stored in the desired
// backend database.  Allowed image formats are those supported by Go's
// standard library image package.  Unrecognized formats of any kind (even
// non-image based) are stored in UnsupportedDir.
type Library struct {
	Path string
}

// New creates and initializes a new library.  All library data is namespaced
// under name in the backend db.  A cache of previously retrieved data is
// maintained up to cacheSize bytes in order to reduce pressure on the db
// backend.
func New(path string) *Library {
}
