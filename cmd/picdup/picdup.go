// picdup identifies and removes duplicate pictures (meta, orig, and
// thumbs) from a library using crypto hashing.
package main

import (
	"flag"
	"log"
	pth "path"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

const cacheSize = 300 * piclib.Mb

var (
	libName = piclib.LibName()
	dry     = flag.Bool("dry", true, "just print output")
)

var hashExists = map[string]bool{}
var lib *piclib.Library

func main() {
	flag.Parse()

	back, err := backend.LoadDefault()
	fatal(err)

	lib, err = piclib.Open(libName, back, cacheSize)
	fatal(err)
	defer lib.Close()

	pics, err := lib.ListPhotos(50000)
	if err != nil {
		log.Print(err)
	}

	for _, p := range pics {
		if hashExists[p.Sha1] {
			removeDup(p, p.Sha1)
		}
		hashExists[p.Sha1] = true
	}
	log.Printf("%v original pics", len(pics))
	log.Printf("%v unique pics", len(hashExists))
}

func removeDup(p *piclib.Photo, sum string) {
	log.Printf("removing photo '%v' with hash '%v'", p.Meta, sum)
	if *dry {
		return
	}

	path := pth.Join(libName, piclib.ImageDir, p.Orig)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(libName, piclib.ThumbDir, p.Thumb2)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(libName, piclib.ThumbDir, p.Thumb1)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(libName, piclib.MetaDir, p.Meta)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
	}
}

func fatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
