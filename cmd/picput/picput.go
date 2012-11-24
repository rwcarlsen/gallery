
package main

import (
	"flag"
	"strings"
	"log"
	"io/ioutil"
	"path/filepath"

	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"launchpad.net/goamz/aws"
)

var amazonS3 = flag.String("amz", "[key-id],[key]", "access piclib on amazon s3")
var local = flag.String("localhd", "[root-dir]", "access piclib on local hd")
var libName = flag.String("lib", "rwc-piclib", "name of library to create/access")

func main() {
	flag.Parse()

	var libs []*piclib.Library
	if strings.Index(*amazonS3, "[") == -1 {
		libs = append(libs, amzLib())
	}
	if strings.Index(*local, "[") == -1 {
		libs = append(libs, localLib())
	}

	picPaths := flag.Args()

	for _, path := range picPaths {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatal(err)
		}

		base := filepath.Base(path)
		for _, lib := range libs {
			lib.AddPhoto(base, data)
		}
	}

	log.Printf("%v photos stored successfully", len(picPaths))
}

func amzLib() *piclib.Library {
	keys := strings.Split(*amazonS3, ",")
	if len(keys) != 2 {
		log.Fatalf("invalid amazon aws keyset '%v'", *amazonS3)
	}

	auth := aws.Auth{AccessKey: keys[0], SecretKey: keys[1]}
	db := amz.New(auth, aws.USEast)
	return piclib.New(*libName, db)
}

func localLib() *piclib.Library {
	db := &localhd.Backend{Root: *local}
	return piclib.New(*libName, db)
}
