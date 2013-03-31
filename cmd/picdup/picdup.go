// picdup identifies and removes duplicate pictures (meta, orig, and
// thumbs) from a library using crypto hashing.
package main

import (
	"flag"
	"log"
	"os"
	pth "path"
	"path/filepath"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

var confPath = filepath.Join(os.Getenv("HOME"), ".backends")

const cacheSize = 300 * piclib.Mb

var (
	libName = flag.String("lib", "rwc-piclib", "name of library to create/access")
	db      = flag.String("db", "", "name of db")
	dry     = flag.Bool("dry", true, "just print output")
)

var hashExists = map[string]bool{}
var lib *piclib.Library

func main() {
	flag.Parse()

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

	path := pth.Join(*libName, piclib.ImageDir, p.Orig)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(*libName, piclib.ThumbDir, p.Thumb2)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(*libName, piclib.ThumbDir, p.Thumb1)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(*libName, piclib.MetaDir, p.Meta)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
	}
}
