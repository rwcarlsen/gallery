
package main

import (
	"log"
	"io/ioutil"
	"github.com/rwcarlsen/gallery/library"
	"github.com/rwcarlsen/gallery/library/local"
	"github.com/rwcarlsen/gallery/library/amz"
	"launchpad.net/goamz/aws"
)

const (
	root = "./"
	sample = "./sample.jpg"
	libName = "rwc-testphotolib"
)

func main() {
	testAmz()
	//testLocal()
}

func testAmz() {


	// setup storage and library
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
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

func testLocal() {
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
