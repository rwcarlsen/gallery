package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

var confPath = filepath.Join(os.Getenv("HOME"), ".backends")

const cacheSize = 300 * piclib.Mb

var (
	libName = flag.String("lib", "rwc-piclib", "name of library to create/access")
	db      = flag.String("db", "", "name of db")
)

var lib *piclib.Library

var l = log.New(os.Stdout, "[picvalid] ", 0)

func main() {
	flag.Parse()

	f, err := os.Open(confPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	set, err := backend.LoadSpecList(f)
	if err != nil {
		log.Fatal(err)
	}

	back, err := set.Make(*db)
	if err != nil {
		log.Fatal(err)
	}

	lib, err = piclib.Open(*libName, back, cacheSize)
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
