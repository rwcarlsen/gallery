// pic-orient adds exif orientation data to all pics in an existing piclib
package main

import (
	"bytes"
	"flag"
	"log"

	"github.com/rwcarlsen/gallery/conf"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goexif/exif"
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
