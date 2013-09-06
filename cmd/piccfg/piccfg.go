package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/rwcarlsen/gallery/conf"
)

var raw = flag.Bool("raw", false, "true to only show explicit config values (no defaults)")

func main() {
	flag.Parse()

	printSpec := func() {
		s, _ := conf.Default.Spec()
		data, _ := json.MarshalIndent(s, "", "    ")
		fmt.Printf("Spec:%s", data)
	}

	if *raw {
		fmt.Printf("BackendSpecPath: %v", conf.Default.BackendSpecPath)
		printSpec()
		fmt.Printf("LibraryName: %v", conf.Default.LibraryName)
		fmt.Printf("LogPath: %v", conf.Default.LogPath)
		fmt.Printf("WebpicsPath: %v", conf.Default.WebpicsPath)
	} else {
		fmt.Printf("BackendSpecPath: %v", conf.Default.SpecPath())
		printSpec()
		fmt.Printf("LibraryName: %v", conf.Default.LibName())
		fmt.Printf("LogPath: %v", conf.Default.LogFile())
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Print("WebpicsPath: <Not Specified>")
				}
			}()
			fmt.Printf("WebpicsPath: %v", conf.Default.WebpicsAssets())
		}()
	}
}
