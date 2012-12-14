
// picput recursively walks passed dirs and photos and adds them to a library
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"encoding/json"
	"io/ioutil"

	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

var db = flag.String("db", "", "backend containing piclib to dump to")
var libName = flag.String("lib", "rwc-piclib", "name of library to create/access")

const cacheSize = 300 * piclib.Mb
const confPath = "/home/robert/.backends"

var validFmt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".png":  true,
	".tif":  true,
	".tiff": true,
	".bmp":  true,
	".exif": true,
	".giff": true,
	".raw":  true,
	".avi":  true,
	".mpg":  true,
	".mp4":  true,
	".mov":  true,
}

var lib *piclib.Library

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

	if spec, ok := dblist[*db]; ok {
		if db, err := spec.Make(); err != nil {
			log.Fatal(err)
		} else {
			lib = piclib.New(*libName, db, cacheSize)
		}
	} else {
		log.Fatalf("db %v not found", *db)
	}

	picPaths := flag.Args()

	for _, path := range picPaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if err := filepath.Walk(path, walkFn); err != nil {
				log.Print(err)
			}
		} else {
			addToLib(path)
		}
	}
}

func walkFn(path string, info os.FileInfo, err error) error {
	if err != nil {
		log.Print(err)
		return nil
	}
	if !info.IsDir() {
		addToLib(path)
	}
	return nil
}

func addToLib(path string) {
	if !validFmt[strings.ToLower(filepath.Ext(path))] {
		log.Printf("skipped file %v", path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		log.Printf("path %v: %v", path, err)
		return
	}

	base := filepath.Base(path)
	if _, err = lib.AddPhoto(base, f); err != nil {
		log.Printf("path %v: %v", path, err)
	}
}

