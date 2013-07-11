// pic-orient adds exif orientation data to all pics in an existing piclib
package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goexif/exif"
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
	defer f.Close()

	set, err := backend.LoadSpecList(f)
	if err != nil {
		log.Fatal(err)
	}

	back, err := set.Make(*db)
	if err != nil {
		log.Fatal(err)
	}

	lib, err = piclib.Open(*libName, back, cacheSize)
	if err != nil {
		log.Fatal(err)
	}
	defer lib.Close()

	// retrieve all pics
	pics, err := lib.ListPhotos(50000)
	if err != nil {
		log.Print(err)
	}

	for _, p := range pics {
		p.Orientation = 1
		if data, err := p.GetOriginal(); err != nil {
			log.Print(err)
		} else {
			if x, err := exif.Decode(bytes.NewBuffer(data)); err == nil {
				if tg, err := x.Get("Orientation"); err == nil {
					p.Orientation = int(tg.Int(0))
				}
			}
		}

		if err := lib.UpdatePhoto(p); err != nil {
			log.Print(err)
		}
	}
}
