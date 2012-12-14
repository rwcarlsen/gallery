
// picput recursively walks passed dirs and photos and adds them to a library
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goamz/aws"
)

var amazonS3 = flag.String("amz", "[key-id],[key]", "access piclib on amazon s3")
var local = flag.String("localhd", "[root-dir]", "access piclib on local hd")
var libName = flag.String("lib", "rwc-piclib", "name of library to create/access")

const cacheSize = 300 * piclib.Mb

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
var errlog = log.New(os.Stdin, "", log.LstdFlags)

func main() {
    flag.Parse()
	if strings.Index(*amazonS3, "[") == -1 {
		lib = amzLib()
	} else if strings.Index(*local, "[") == -1 {
		lib = localLib()
	} else {
		log.Fatal("no library specified")
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
		errlog.Printf("skipped file %v", path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		errlog.Printf("path %v: %v", path, err)
		return
	}

	base := filepath.Base(path)
	if _, err = lib.AddPhoto(base, f); err != nil {
		errlog.Printf("path %v: %v", path, err)
	}
}

func amzLib() *piclib.Library {
	keys := strings.Split(*amazonS3, ",")
	if len(keys) != 2 {
		log.Fatalf("invalid amazon aws keyset '%v'", *amazonS3)
	}

	auth := aws.Auth{AccessKey: keys[0], SecretKey: keys[1]}
	db := amz.New(auth, aws.USEast)
	return piclib.New(*libName, db, cacheSize)
}

func localLib() *piclib.Library {
	db := &localhd.Interface{Root: *local}
	return piclib.New(*libName, db, cacheSize)
}
