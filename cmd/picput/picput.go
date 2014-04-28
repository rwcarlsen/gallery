// picput recursively walks passed dirs and photos and adds them to a library.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/gallery/conf"
	"github.com/rwcarlsen/gallery/piclib"
)

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
	".m4v":  true,
}

var lib *piclib.Library

func main() {
	log.SetFlags(0)
	flag.Parse()

	back, err := conf.Default.Backend()
	if err != nil {
		log.Fatal(err)
	}

	lib, err = piclib.Open(conf.Default.LibName(), back, cacheSize)
	if err != nil {
		log.Fatal(err)
	}
	defer lib.Close()

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
	} else if info.IsDir() {
		return nil
	} else if strings.Index(path, piclib.NameSep) != -1 {
		return fmt.Errorf("filename '%v' contains reserved sequence '%v'", path, piclib.NameSep)
	}

	addToLib(path)
	return nil
}

func addToLib(path string) {
	if !validFmt[strings.ToLower(filepath.Ext(path))] {
		fmt.Printf("[SKIP] file %v not a supported type\n", path)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		log.Printf("path %v: %v", path, err)
		return
	}
	defer f.Close()

	base := filepath.Base(path)
	if _, err = lib.AddPhoto(base, f); err != nil {
		if _, ok := err.(piclib.DupErr); ok {
			fmt.Printf("[SKIP] %v\n", err)
		} else {
			log.Printf("[ERROR] '%v': %v", path, err)
		}
	} else {
		fmt.Printf("file %v added\n", path)
	}
}
