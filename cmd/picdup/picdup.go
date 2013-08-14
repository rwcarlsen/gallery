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

// map[hash]filename
var hashes = map[string]string{}
var lib *piclib.Library

func main() {
	flag.Parse()
	log.SetPrefix("[picdup] ")
	log.SetFlags(0)

	back, err := backend.LoadDefault()
	fatal(err)

	lib, err = piclib.Open(libName, back, cacheSize)
	fatal(err)
	defer lib.Close()

	pics, err := lib.ListPhotos(1000000)
	if err != nil {
		log.Print(err)
	}

	for _, p := range pics {
		if fname, ok := hashes[p.Sha1]; ok {
			removeDup(p, fname)
			continue
		}
		hashes[p.Sha1] = p.Orig
	}
	log.Printf("%v original pics", len(pics))
	log.Printf("%v unique pics", len(hashes))
	log.Printf("%v duplicate pics", len(pics) - len(hashes))
}

func removeDup(p *piclib.Photo, fname string) {
	log.Printf("removed photo '%v' as duplicate of '%v'", p.Orig, fname)
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
