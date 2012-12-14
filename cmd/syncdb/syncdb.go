package main

import (
	"flag"
	"log"
	"strings"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goamz/aws"
)

var amazonS3 = flag.String("amz", "[key-id],[key]", "access piclib on amazon s3")
var local = flag.String("localhd", "[root-dir]", "access piclib on local hd")
var from = flag.String("flow", "toamz", "")
var to = flag.String("flow", "toamz", "")

var syncPath = flag.String("path", "", "name of library to create/access")


var dry = flag.Bool("dry", false, "true to just print output of command and not sync anything")
var del = flag.Bool("del", false, "delete files at dst that don't exist at src")

func main() {
	flag.Parse()

	var dbs []backend.Interface
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
		config = backend.Sdry
	}
	if *del {
		config = config | backend.Sdel
	}

	var err error
	var results []string
	switch *flow {
	case "toamz":
		results, err = backend.OneWay(*syncPath, config, dbs[1], dbs[0])
	case "fromamz":
		results, err = backend.OneWay(*syncPath, config, dbs[0], dbs[1])
	case "allway":
		results, err = backend.AllWay(*syncPath, config, dbs...)
	default:
		log.Fatalf("invalid flow %v", *flow)
	}

	if err != nil {
		log.Println(err)
	}
	for _, r := range results {
		log.Println(r)
	}
}

func amzLib() backend.Interface {
	keys := strings.Split(*amazonS3, ",")
	if len(keys) != 2 {
		log.Fatalf("invalid amazon aws keyset '%v'", *amazonS3)
	}
	auth := aws.Auth{AccessKey: keys[0], SecretKey: keys[1]}
	db := amz.New(auth, aws.USEast)
	db.DbName = "amz"
	return db
}

func localLib() backend.Interface {
	return &localhd.Interface{Root: *local, DbName: "localhd"}
}
