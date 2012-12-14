
// picdup identifies and removes duplicate pictures (meta, orig, and
// thumbs) from a library using crypto hashing.
package main

import (
	"log"
	"fmt"
	"os"
	"flag"
	"path/filepath"
	"crypto"
	_ "crypto/sha1"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/backend/localhd"
)


const backendPath = "/media/spare"
const libName = "rwc-piclib"

var dry = flag.Bool("dry", true, "just print output")

var h = crypto.SHA1.New()
var hashExists = map[string]bool{}
var lib *piclib.Library

func main() {
	flag.Parse()

	db := &localhd.Backend{Root: backendPath}
	lib = piclib.New(libName, db, 100 * piclib.Mb)

	pics, err := lib.ListPhotos(50000)
	if err != nil {
		log.Print(err)
	}

	log.Printf("len(pics)=%v", len(pics))
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
	log.Printf("len(hashExists)=%v", len(hashExists))
}

func removeDup(p *piclib.Photo, sum string) {
	log.Printf("removing photo '%v' with hash '%v'", p.Meta, sum)
	if *dry {
		return
	}
	root := filepath.Join(backendPath, libName)

	path := filepath.Join(root, piclib.ImageDir, p.Orig)
	if err := os.Remove(path); err != nil {
		log.Print(err)
		return
	}

	path = filepath.Join(root, piclib.ThumbDir, p.Thumb2)
	if err := os.Remove(path); err != nil {
		log.Print(err)
		return
	}

	path = filepath.Join(root, piclib.ThumbDir, p.Thumb1)
	if err := os.Remove(path); err != nil {
		log.Print(err)
		return
	}

	path = filepath.Join(root, piclib.MetaDir, p.Meta)
	if err := os.Remove(path); err != nil {
		log.Print(err)
	}
}
