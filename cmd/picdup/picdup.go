// picdup identifies and removes duplicate pictures (meta, orig, and
// thumbs) from a library using crypto hashing.
package main

import (
	"flag"
	"fmt"
	"log"
	pth "path"

	"github.com/rwcarlsen/gallery/conf"
	"github.com/rwcarlsen/gallery/piclib"
)

const cacheSize = 300 * piclib.Mb

var (
	libName = conf.Default.LibName()
	dry     = flag.Bool("dry", true, "just print output")
)

// map[hash]filename
var hashes = map[string]string{}
var lib *piclib.Library

func main() {
	flag.Parse()
	log.SetFlags(0)

	back, err := conf.Default.Backend()
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
	fmt.Printf("%v original pics", len(pics))
	fmt.Printf("%v unique pics", len(hashes))
	fmt.Printf("%v duplicate pics", len(pics)-len(hashes))
	if *dry {
		fmt.Printf("0 pics removed")
	} else {
		fmt.Printf("%v pics removed", len(pics)-len(hashes))
	}
}

func removeDup(p *piclib.Photo, fname string) {
	fmt.Printf("'%v' duplicate of '%v'", p.Orig, fname)
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
