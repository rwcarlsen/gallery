
package main

import (
	"log"
	"io/ioutil"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/piclib/local"
	"github.com/rwcarlsen/gallery/piclib/amz"
	"launchpad.net/goamz/aws"
)

const (
	root = "./"
	sample = "./sample.jpg"
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
	lib := piclib.New(libName, db)

	// retrieve file
	p, err := lib.GetPhoto("2012-04-01-22-13-55-sample.json")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(p)
}

func testLocal() {
	// setup storage and piclib
	db := &local.LocalBack{Root: root}
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
