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
		fmt.Printf("Spec:%s\n", data)
	}

	if *raw {
		fmt.Printf("BackendSpecPath: %v\n", conf.Default.BackendSpecPath)
		printSpec()
		fmt.Printf("LibraryName: %v\n", conf.Default.LibraryName)
		fmt.Printf("LogPath: %v\n", conf.Default.LogPath)
		fmt.Printf("WebpicsPath: %v\n", conf.Default.WebpicsPath)
	} else {
		fmt.Printf("BackendSpecPath: %v\n", conf.Default.SpecPath())
		printSpec()
		fmt.Printf("LibraryName: %v\n", conf.Default.LibName())
		fmt.Printf("LogPath: %v\n", conf.Default.LogFile())
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Print("WebpicsPath: <Not Specified>\n")
				}
			}()
			fmt.Printf("WebpicsPath: %v\n", conf.Default.WebpicsAssets())
		}()
	}
}
