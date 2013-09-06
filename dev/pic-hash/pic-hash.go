// pic-hash adds sha1 and size data to all pics in an existing piclib
package main

import (
	"bytes"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"log"

	"github.com/rwcarlsen/gallery/conf"
	"github.com/rwcarlsen/gallery/piclib"
)

const cacheSize = 300 * piclib.Mb

var lib *piclib.Library

func main() {
	flag.Parse()

	back, err := conf.Default.Backend()
	if err != nil {
		log.Fatal(err)
	}

	lib, err = piclib.Open(conf.Default.LibName(), back, cacheSize)
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
