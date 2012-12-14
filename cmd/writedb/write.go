package main

import (
	"flag"
	"io/ioutil"
	"log"
	"bytes"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/piclib"
	"launchpad.net/goamz/aws"
)

var del := flag.Bool("del", false, "delete instead of overwrite the specified file")

func main() {
	flag.Parse()
	srcPath := flag.Arg(0)
	dstPath := flag.Arg(1)

	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		log.Fatal(err)
	}

	db := amzLib()
	if err := db.Put(dstPath, bytes.NewReader(data)); err != nil {
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
