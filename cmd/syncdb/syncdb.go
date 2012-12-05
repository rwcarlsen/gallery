
package main

import (
	"flag"
	"log"
	"strings"

	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/backend/dbsync"
	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"launchpad.net/goamz/aws"
)

var amazonS3 = flag.String("amz", "[key-id],[key]", "access piclib on amazon s3")
var local = flag.String("localhd", "[root-dir]", "access piclib on local hd")
var syncPath = flag.String("path", "", "name of library to create/access")
var dry = flag.Bool("dry", false, "true to just print output of command and not sync anything")
var flow = flag.String("flow", "toamz", "")

func main() {
	flag.Parse()

	var dbs []piclib.Backend
	if strings.Index(*amazonS3, "[") == -1 {
		dbs = append(dbs, amzLib())
	}
	if strings.Index(*local, "[") == -1 {
		dbs = append(dbs, localLib())
	}

	if len(dbs) < 2 {
		log.Fatal("not enough backends")
	}

	config := 0
	if *dry {
		config = dbsync.Cdry
	}

	var err error
	var results []string
	if *flow == "toamz" {
		results, err = dbsync.OneWay(*syncPath, config, dbs[1], dbs[0])
	} else if *flow == "allway" {
		results, err = dbsync.AllWay(*syncPath, config, dbs...)
	} else {
		log.Fatalf("invalid flow %v", *flow)
	}

	if err != nil {
		log.Println(err)
	}
	for _, r := range results {
		log.Println(r)
	}
}

func amzLib() piclib.Backend {
	keys := strings.Split(*amazonS3, ",")
	if len(keys) != 2 {
		log.Fatalf("invalid amazon aws keyset '%v'", *amazonS3)
	}
	auth := aws.Auth{AccessKey: keys[0], SecretKey: keys[1]}
	db := amz.New(auth, aws.USEast)
	db.DbName = "amz"
	return db
}

func localLib() piclib.Backend {
	return &localhd.Backend{Root: *local, DbName: "localhd"}
}
