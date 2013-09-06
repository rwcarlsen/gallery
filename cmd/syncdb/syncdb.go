// syncdb performs a uni-directional sync between two databases.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/rwcarlsen/gallery/backend"
)

var from = flag.String("from", "", "path to source spec")
var to = flag.String("to", "", "path to dst spec")
var syncPath = flag.String("path", "", "name of library to create/access")

var dry = flag.Bool("dry", false, "true to just print output of command and not sync anything")
var del = flag.Bool("del", false, "delete files at dst that don't exist at src")

func must(s *backend.Spec, err error) backend.Interface {
	if err != nil {
		log.Fatal(err)
	}
	back, err := s.Make()
	if err != nil {
		panic(err)
	}
	return back
}

func main() {
	flag.Parse()

	f1, err := os.Open(*from)
	if err != nil {
		log.Fatal(err)
	}
	defer f1.Close()

	f2, err := os.Open(*to)
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()

	fromDb := must(backend.LoadSpec(f1))
	toDb := must(backend.LoadSpec(f2))

	config := 0
	if *dry {
		config = backend.SyncDry
	}
	if *del {
		config = config | backend.SyncDel
	}

	results, err := backend.SyncOneWay(*syncPath, config, fromDb, toDb)
	if err != nil {
		log.Println(err)
	}
	for _, r := range results {
		log.Println(r)
	}
}
