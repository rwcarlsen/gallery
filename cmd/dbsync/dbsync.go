
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

var libs = make(map[string]*dbInfo)

type dbInfo struct {
	db piclib.Backend
	objects map[string]bool
}

func main() {
	flag.Parse()

	if strings.Index(*amazonS3, "[") == -1 {
		libs["amz"] = amzLib()
	}
	if strings.Index(*local, "[") == -1 {
		libs["localhd"] = localLib()
	}

	// retrieve file list for each db
	for _, info := range libs {
		names, err := info.db.ListN(*libName, 0)
		if err != nil {
			log.Fatal(err)
		}

		for _, name := range names {
			info.objects[name] = true
		}
	}

	// sync them - add only - no mod checks
	for n1, info1 := range libs {
		for n2, info2 := range libs {
			for name, _ := range info1.objects {
				if !info2.objects[name] {
					log.Printf("sync from %v to %v: %v", n1, n2, name)
					data, err := info1.db.Get(name)
					if err != nil {
						log.Print(err)
						continue
					}
					if err := info2.db.Put(name, data); err != nil {
						log.Print(err)
					}
				}
			}
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
