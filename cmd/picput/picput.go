
package main

import (
	"os"
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

var validFmt = map[string]bool{
	".jpg": true,
	".jpeg": true,
	".gif": true,
	".png": true,
	".tif": true,
	".tiff": true,
	".bmp": true,
	".exif": true,
	".giff": true,
	".raw": true,
	".avi": true,
	".mpg": true,
	".mp4": true,
	".mov": true,
}

var libs []*piclib.Library
var errlog = log.New(os.Stdin, "", log.LstdFlags)

var done = make(chan bool)
var count = 0

func main() {
	flag.Parse()

	if strings.Index(*amazonS3, "[") == -1 {
		libs = append(libs, amzLib())
	}
	if strings.Index(*local, "[") == -1 {
		libs = append(libs, localLib())
	}

	picPaths := flag.Args()

	for _, path := range picPaths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if err := filepath.Walk(path, walkFn); err != nil {
				log.Print(err)
			}
		} else {
			addToLibs(path)
		}
	}

	for count > 0 {
		<-done
		count--
	}
}

func walkFn(path string, info os.FileInfo, err error) error {
	if err != nil {
		log.Print(err)
		return nil
	}
	if !info.IsDir() {
		count++
		go addToLibs(path)
	}
	return nil
}

func addToLibs(path string) {
	if !validFmt[strings.ToLower(filepath.Ext(path))] {
		errlog.Printf("skipped file %v", path)
		return
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		errlog.Print(err)
		return
	}

	base := filepath.Base(path)
	for _, lib := range libs {
		if _, err = lib.AddPhoto(base, data); err != nil {
			errlog.Print(err)
		}
	}
	done <- true
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
