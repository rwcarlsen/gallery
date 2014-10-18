package main

import (
	"bytes"
	"database/sql"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"path/filepath"

	"github.com/disintegration/imaging"
	_ "github.com/mxk/go-sqlite/sqlite3"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goexif/exif"
)

var libpath = flag.String("lib", piclib.DefaultPath(), "path to picture library")

func main() {
	flag.Parse()

	lib, err := piclib.Open(*libpath)
	if err != nil {
		log.Fatal(err)
	}

	pics, err := lib.List(0, 0)
	if err != nil {
		log.Fatal(err)
	}

	dbpath := filepath.Join(*libpath, piclib.Libname)
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for i, p := range pics {
		fmt.Printf("fixing thumb %v (%v)\n", i, p.Name)
		data, err := p.Thumb()
		if err != nil {
			log.Fatal(err)
		}
		thumb, _, err := image.Decode(bytes.NewBuffer(data))
		if err != nil {
			continue
		}

		orient := getOrient(p)

		switch orient {
		case 3, 4:
			thumb = imaging.Rotate180(thumb)
		case 5, 6:
			thumb = imaging.Rotate270(thumb)
		case 7, 8:
			thumb = imaging.Rotate90(thumb)
		}

		switch orient {
		case 2, 5, 4, 7:
			thumb = imaging.FlipH(thumb)
		}

		var buf bytes.Buffer
		err = jpeg.Encode(&buf, thumb, nil)
		if err != nil {
			log.Fatal(err)
		}

		_, err = db.Exec("UPDATE files SET thumb = ?,orient = ? WHERE id = ?;", buf.Bytes(), orient, p.Id)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func getOrient(p *piclib.Pic) int {
	orig, err := p.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer orig.Close()

	x, err := exif.Decode(orig)
	if err == nil {
		tag, err := x.Get(exif.Orientation)
		if err == nil {
			v, _ := tag.Int(0)
			return int(v)
		}
	}
	return 0
}
