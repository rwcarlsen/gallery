
package main

import (
	"flag"
	"log"
	"io/ioutil"

	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/backend/amz"
	"launchpad.net/goamz/aws"
)

func main() {
	flag.Parse()

	flag.Args()

	// setup storage and piclib
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	lib := piclib.New(libName, db)

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
