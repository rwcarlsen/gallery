// picput recursively walks passed dirs and photos and adds them to a library.
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/rwcarlsen/gallery/piclib"
)

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

var lib = flag.String("lib", "", "path to picture library (blank => $HOME/piclib")

func main() {
	log.SetFlags(0)
	flag.Parse()

	if *lib == "" {
		*lib = piclib.Path()
	}

	files := flag.Args()
	if len(flag.Args()) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		files = strings.Split(string(data), "\n", -1)
	}

	for _, path := range files {
		// ...
	}
}
