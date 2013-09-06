package main

import (
	"encoding/json"
	"flag"
	"log"

	"github.com/rwcarlsen/gallery/conf"
)

var raw = flag.Bool("raw", false, "true to only show explicit config values (no defaults)")

func main() {
	flag.Parse()
	log.SetPrefix("[piccfg] ")
	log.SetFlags(0)

	printSpec := func() {
		s, _ := conf.Default.Spec()
		data, _ := json.MarshalIndent(s, "[piccfg] ", "    ")
		log.Printf("Spec:%s", data)
	}

	if *raw {
		log.Printf("BackendSpecPath: %v", conf.Default.BackendSpecPath)
		printSpec()
		log.Printf("LibraryName: %v", conf.Default.LibraryName)
		log.Printf("LogPath: %v", conf.Default.LogPath)
		log.Printf("WebpicsPath: %v", conf.Default.WebpicsPath)
	} else {
		log.Printf("BackendSpecPath: %v", conf.Default.SpecPath())
		printSpec()
		log.Printf("LibraryName: %v", conf.Default.LibName())
		log.Printf("LogPath: %v", conf.Default.LogFile())
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Print("WebpicsPath: <Not Specified>")
				}
			}()
			log.Printf("WebpicsPath: %v", conf.Default.WebpicsAssets())
		}()
	}
}
