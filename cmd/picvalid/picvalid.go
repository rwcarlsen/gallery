package main

import (
	"flag"
	"fmt"
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

	pics, err := lib.ListPhotos(50000)
	if err != nil {
		log.Printf("photo listing error: %v\n", err)
	}

	for _, p := range pics {
		valid, err := p.Verify()
		if err != nil {
			log.Printf("failed to verify photo '%v': %v\n", p.Orig, err)
		} else if !valid {
			fmt.Printf("ERROR: photo '%v' is corrupt.\n", p.Orig)
		} else {
			fmt.Printf("VALID: photo '%v' verified.\n", p.Orig)
		}
	}
}
