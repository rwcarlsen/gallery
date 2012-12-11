package main

import (
	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"launchpad.net/goamz/aws"
	"log"
	"os"
)

const (
	root    = "./"
	sample  = "./sample.jpg"
	libName = "rwc-webpics"
)

func main() {
	//testAmzPut()
	//testAmzGet()
	testAmzListN()
	//testLocal()
}

func testAmzPut() {
	// setup storage and piclib
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	lib := piclib.New(libName, db, 100 * piclib.Mb)

	f, err := os.Open(sample)
	if err != nil {
		log.Fatal(err)
	}

	// load and dump photo, thumbs, etc.
	p, err := lib.AddPhoto(sample, f)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(p)
}

func testAmzListN() {
	// setup storage and piclib
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	names, err := db.ListN("rwc-webpics/metadata", 5)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(names)
}

func testAmzGet() {
	// setup storage and piclib
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	lib := piclib.New(libName, db, 100 * piclib.Mb)

	// retrieve file
	p, err := lib.GetPhoto("2012-04-01-22-13-55-sample.json")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(p)
}

func testLocal() {
	// setup storage and piclib
	db := &localhd.Backend{Root: root}
	lib := piclib.New(libName, db, 100 * piclib.Mb)

	f, err := os.Open(sample)
	if err != nil {
		log.Fatal(err)
	}

	// load and dump photo, thumbs, etc.
	p, err := lib.AddPhoto(sample, f)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(p)
}
