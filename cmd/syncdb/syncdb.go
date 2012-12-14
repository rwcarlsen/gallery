package main

import (
	"flag"
	"log"
	"encoding/json"
	"io/ioutil"

	"github.com/rwcarlsen/gallery/backend"
)

var from = flag.String("from", "", "source backend")
var to = flag.String("to", "", "destination backend")
var syncPath = flag.String("path", "", "name of library to create/access")

var dry = flag.Bool("dry", false, "true to just print output of command and not sync anything")
var del = flag.Bool("del", false, "delete files at dst that don't exist at src")

const confPath = "/home/robert/.backends"

func must(b backend.Interface, err error) backend.Interface {
	if err != nil {
		log.Fatal(err)
	}
	return b
}

func main() {
	flag.Parse()

	data, err := ioutil.ReadFile(confPath)
	if err != nil {
		log.Fatal(err)
	}

	dblist := map[string]*backend.Spec{}
	if err := json.Unmarshal(data, &dblist); err != nil {
		log.Fatal(err)
	}

	var fromDb, toDb backend.Interface
	if spec, ok := dblist[*from]; ok {
		fromDb = must(spec.Make())
	}
	if spec, ok := dblist[*to]; ok {
		toDb = must(spec.Make())
	}

	if fromDb == nil || toDb == nil {
		log.Fatal("named db(s) not found")
	}

	config := 0
	if *dry {
		config = backend.Sdry
	}
	if *del {
		config = config | backend.Sdel
	}

	results, err := backend.SyncOneWay(*syncPath, config, fromDb, toDb)
	if err != nil {
		log.Println(err)
	}
	for _, r := range results {
		log.Println(r)
	}
}

