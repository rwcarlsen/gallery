package main

import (
	"flag"
	"log"
	"os"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

const cacheSize = 300 * piclib.Mb

var lib *piclib.Library

var l = log.New(os.Stdout, "[picvalid] ", 0)

func main() {
	flag.Parse()

	back, err := backend.LoadDefault()
	if err != nil {
		log.Fatal(err)
	}

	lib, err = piclib.Open(piclib.LibName(), back, cacheSize)
	if err != nil {
		log.Fatal(err)
	}
	defer lib.Close()

	pics, err := lib.ListPhotos(50000)
	if err != nil {
		log.Printf("photo listing error: %v", err)
	}

	for _, p := range pics {
		valid, err := p.Verify()
		if err != nil {
			log.Printf("failed to verify photo '%v': %v", p.Orig, err)
		} else if !valid {
			l.Printf("ERROR: photo '%v' is corrupt.", p.Orig)
		} else {
			l.Printf("VALID: photo '%v' verified.", p.Orig)
		}
	}
}
