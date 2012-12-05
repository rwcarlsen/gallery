package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"launchpad.net/goamz/aws"
)

const nWorkers = 5

var amazonS3 = flag.String("amz", "[key-id],[key]", "access piclib on amazon s3")
var local = flag.String("localhd", "[root-dir]", "access piclib on local hd")
var libName = flag.String("lib", "rwc-piclib", "name of library to create/access")

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

var doneCh = make(chan bool)
var inCh = make(chan string)
var count = 1

func main() {
	flag.Parse()

	if strings.Index(*amazonS3, "[") == -1 {
		lib = amzLib()
	} else if strings.Index(*local, "[") == -1 {
		lib = localLib()
	}

	if lib == nil {
		return
	}

	picPaths := flag.Args()

	for i := 0; i < nWorkers; i++ {
		go addToLib()
	}

	go func() {
		for _, path := range picPaths {
			if info, err := os.Stat(path); err == nil && info.IsDir() {
				if err := filepath.Walk(path, walkFn); err != nil {
					log.Print(err)
				}
			} else {
				count++
				inCh <- path
			}
		}
		doneCh <- true
	}()

	for count > 0 {
		<-doneCh
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
		inCh <- path
	}
	return nil
}

func addToLib() {
	for {
		path := <-inCh
		if !validFmt[strings.ToLower(filepath.Ext(path))] {
			errlog.Printf("skipped file %v", path)
			doneCh <- true
			continue
		}

		data, err := ioutil.ReadFile(path)
		if err != nil {
			errlog.Printf("path %v: %v", path, err)
			doneCh <- true
			continue
		}

		base := filepath.Base(path)
		if _, err = lib.AddPhoto(base, data); err != nil {
			errlog.Printf("path %v: %v", path, err)
		}
		doneCh <- true
	}
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
