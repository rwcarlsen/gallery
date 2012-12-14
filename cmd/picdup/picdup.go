
// picdup identifies and removes duplicate pictures (meta, orig, and
// thumbs) from a library using crypto hashing.
package main

import (
	"log"
	"fmt"
	"flag"
	pth "path"
	"crypto"
	_ "crypto/sha1"
	"strings"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goamz/aws"
)

const cacheSize = 300 * piclib.Mb

var amazonS3 = flag.String("amz", "[key-id],[key]", "access piclib on amazon s3")
var local = flag.String("localhd", "[root-dir]", "access piclib on local hd")
var libName = flag.String("lib", "rwc-piclib", "name of library to create/access")
var dry = flag.Bool("dry", true, "just print output")

var h = crypto.SHA1.New()
var hashExists = map[string]bool{}
var lib *piclib.Library

func main() {
	flag.Parse()
	if strings.Index(*amazonS3, "[") == -1 {
		lib = amzLib()
	} else if strings.Index(*local, "[") == -1 {
		lib = localLib()
	} else {
		log.Fatal("no library specified")
	}

	pics, err := lib.ListPhotos(50000)
	if err != nil {
		log.Print(err)
	}

	for _, p := range pics {
		data, err := p.GetOriginal()
		if err != nil {
			log.Print(err)
			continue
		}

		if n, err := h.Write(data); n < len(data) || err != nil {
			log.Printf("n=%v, len(data)=%v, err=%v", n, len(data), err)
			h.Reset()
			continue
		}

		hashSum := fmt.Sprintf("%X", h.Sum([]byte{}))
		h.Reset()

		if hashExists[hashSum] {
			removeDup(p, hashSum)
		}
		hashExists[hashSum] = true
	}
	log.Printf("%v original pics", len(pics))
	log.Printf("%v unique pics", len(hashExists))
}

func removeDup(p *piclib.Photo, sum string) {
	log.Printf("removing photo '%v' with hash '%v'", p.Meta, sum)
	if *dry {
		return
	}

	path := pth.Join(*libName, piclib.ImageDir, p.Orig)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(*libName, piclib.ThumbDir, p.Thumb2)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(*libName, piclib.ThumbDir, p.Thumb1)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
		return
	}

	path = pth.Join(*libName, piclib.MetaDir, p.Meta)
	if err := lib.Db.Del(path); err != nil {
		log.Print(err)
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

