// picput recursively walks passed dirs and photos and adds them to a library.
package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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

type CmdFunc func(cmd string, args []string)

var cmds = map[string]CmdFunc{
	"put": put,
}

func newFlagSet(cmd, args, desc string) *flag.FlagSet {
	fs := flag.NewFlagSet("put", flag.ExitOnError)
	fs.Usage = func() {
		log.Printf("Usage: pic %s [OPTION] %s\n%s\n", cmd, args, desc)
		fs.PrintDefaults()
	}
	return fs
}

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		log.Printf("Usage: pic [opts] <subcommand> [opts] [args]\n")
		flag.PrintDefaults()
		log.Printf("Subcommands:\n")
		for cmd, _ := range cmds {
			log.Printf("  %v", cmd)
		}
	}

	flag.Parse()

	if *lib == "" {
		*lib = piclib.Path()
	}

	if len(flag.Args()) == 0 {
		flag.Usage()
		return
	}

	cmd, ok := cmds[flag.Arg(0)]
	if !ok {
		flag.Usage()
		return
	}
	cmd(flag.Arg(0), flag.Args()[1:])
}

func put(cmd string, args []string) {
	desc := "copies given files to the library path. If no args are given, reads a list of files from stdin."
	fs := newFlagSet("put", "[FILE...]", desc)
	fs.Parse(args)
	rename := fs.Bool("rename", true, "true to rename files with an exif date or sha1 hash prefix")

	files := args
	if len(args) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		files = strings.Fields(string(data))
	}

	for _, path := range files {
		p := strings.TrimSpace(path)
		if p == "" {
			continue
		}

		src, err := os.Open(p)
		if err != nil {
			log.Fatal(err)
		}
		defer src.Close()

		dstpath := filepath.Join(*lib, filepath.Base(p))
		dst, err := os.Create(dstpath)
		if err != nil {
			log.Fatal(err)
		}
		defer src.Close()

		_, err = io.Copy(dst, src)
		if err != nil {
			log.Fatal(err)
		}

		if *rename {
			piclib.Rename(dstpath)
		}
	}
}
