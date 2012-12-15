
// picdup identifies and removes duplicate pictures (meta, orig, and
// thumbs) from a library using crypto hashing.
package main

import (
	"os"
	"log"
	"fmt"
	"flag"
	pth "path"
	"crypto"
	_ "crypto/sha1"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

const cacheSize = 300 * piclib.Mb
const confPath = "/home/robert/.backends"

var libName = flag.String("lib", "rwc-piclib", "name of library to create/access")
var db = flag.String("db", "", "name of db")
var dry = flag.Bool("dry", true, "just print output")

var h = crypto.SHA1.New()
var hashExists = map[string]bool{}
var lib *piclib.Library

func main() {
	flag.Parse()

	f, err := os.Open(confPath)
	if err != nil {
		log.Fatal(err)
	}
	set, err := backend.LoadSpecSet(f)
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
		data, err := p.GetOriginal()
		if err != nil {
			log.Print(err)
			continue
		}

		if n, err := h.Write(data); n < len(data) || err != nil {
			log.Printf("n=%v, len(data)=%v, err=%v", n, len(data), err)
			h.Reset()
			continue
		}

		hashSum := fmt.Sprintf("%X", h.Sum([]byte{}))
		h.Reset()

		if hashExists[hashSum] {
			removeDup(p, hashSum)
		}
		hashExists[hashSum] = true
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

