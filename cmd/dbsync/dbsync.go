
package main

import (
	"flag"
	"log"
	"strings"

	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"launchpad.net/goamz/aws"
)

var amazonS3 = flag.String("amz", "[key-id],[key]", "access piclib on amazon s3")
var local = flag.String("localhd", "[root-dir]", "access piclib on local hd")
var libName = flag.String("lib", "rwc-piclib", "name of library to create/access")

var libs []*dbInfo

type dbInfo struct {
	db piclib.Backend
	objects map[string]bool
}

func main() {
	flag.Parse()

	if strings.Index(*amazonS3, "[") == -1 {
		libs = append(libs, amzLib())
	}
	if strings.Index(*local, "[") == -1 {
		libs = append(libs, localLib())
	}

	for _, info := range libs {
		names, err := info.db.ListN(*libName, 20)
		if err != nil {
			log.Fatal(err)
		}
		for _, name := range names {
			log.Println(name)
		}
	}
}

func amzLib() *dbInfo {
	keys := strings.Split(*amazonS3, ",")
	if len(keys) != 2 {
		log.Fatalf("invalid amazon aws keyset '%v'", *amazonS3)
	}
	auth := aws.Auth{AccessKey: keys[0], SecretKey: keys[1]}
	return &dbInfo{amz.New(auth, aws.USEast), make(map[string]bool)}
}

func localLib() *dbInfo {
	return &dbInfo{&localhd.Backend{Root: *local}, make(map[string]bool)}
}
