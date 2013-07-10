// pic-hash adds sha1 and size data to all pics in an existing piclib
package main

import (
	"bytes"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

var db = flag.String("db", "", "backend containing piclib to dump to")
var libName = flag.String("lib", "testlib", "name of library to create/access")

var confPath = filepath.Join(os.Getenv("HOME"), ".backends")

const cacheSize = 300 * piclib.Mb

var lib *piclib.Library

func main() {
	flag.Parse()

	// create library from backend spec
	f, err := os.Open(confPath)
	if err != nil {
		log.Fatal(err)
	}
	set, err := backend.LoadSpecList(f)
	if err != nil {
		log.Fatal(err)
	}

	back, err := set.Make(*db)
	if err != nil {
		log.Fatal(err)
	}

	lib = piclib.New(*libName, back, cacheSize)

	// retrieve all pics
	pics, err := lib.ListPhotos(50000)
	if err != nil {
		log.Print(err)
	}

	for _, p := range pics {
		if data, err := p.GetOriginal(); err != nil {
			log.Print(err)
		} else {
			sum, n := hash(bytes.NewReader(data))
			p.Size = int(n)
			p.Sha1 = sum
		}

		if err := lib.UpdatePhoto(p); err != nil {
			log.Print(err)
		}
	}
}

func hash(r io.ReadSeeker) (sum string, n int64) {
	r.Seek(0, 0)
	h := sha1.New()
	var err error
	if n, err = io.Copy(h, r); err != nil {
		return "FailedHash", n
	}
	return fmt.Sprintf("%X", h.Sum(nil)), n
}
