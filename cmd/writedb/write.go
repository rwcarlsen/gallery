
package main

import (
	"log"
	"flag"
	"io/ioutil"

	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/backend/amz"
	"launchpad.net/goamz/aws"
)

func main() {
	flag.Parse()
	srcPath := flag.Arg(0)
	dstPath := flag.Arg(1)

	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		log.Fatal(err)
	}

	db := amzLib()
	if err := db.Put(dstPath, data); err != nil {
		log.Fatal(err)
	}
	log.Println("success")
}

func amzLib() piclib.Backend {
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	db.DbName = "amz"
	return db
}

