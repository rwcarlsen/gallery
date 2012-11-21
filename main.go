
package main

import (
	"log"
	"io/ioutil"
	"github.com/rwcarlsen/gallery/library"
	"github.com/rwcarlsen/gallery/library/local"
)

const (
	root = "./"
	sample = "./sample.jpg"
	libName = "testlib"
)

func main() {
	// setup storage and library
	db := &local.LocalBack{Root: root}
	lib := library.New(libName, db)

	data, err := ioutil.ReadFile(sample)
	if err != nil {
		log.Fatal(err)
	}

	// load and dump photo, thumbs, etc.
	p, err := lib.AddPhoto(sample, data)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(p)
}
