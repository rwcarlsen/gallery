// picput recursively walks passed dirs and photos and adds them to a library.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/rwcarlsen/gallery/piclib"
)

var lib = flag.String("lib", "", "path to picture library (blank => env PICLIB => $HOME/piclib)")

type CmdFunc func(cmd string, args []string)

var cmds = map[string]CmdFunc{
	"put":      put,
	"validate": validate,
	"dups":     dups,
	"find":     find,
	"thumb":    thumb,
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

	if *lib != "" {
		var err error
		piclib.Path, err = filepath.Abs(*lib)
		if err != nil {
			log.Fatal(err)
		}
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

func dups(cmd string, args []string) {
	desc := "Print duplicate files from the list. If no args are given, reads a list of files from stdin."
	fs := newFlagSet("dups", "[FILE...]", desc)
	all := fs.Bool("all", false, "true to check every file in the library")
	fs.Parse(args)

	files := fs.Args()
	if *all {
		var err error
		files, err = piclib.List(-1)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(files) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		files = strings.Fields(string(data))
	}

	sums := map[string]string{}
	for _, file := range files {
		sum, err := piclib.Checksum(file)
		if err != nil {
			log.Fatal(err)
		}
		s := fmt.Sprintf("%x", sum)
		if orig, ok := sums[s]; ok {
			fmt.Printf("[DUP] %v is duplicate of %v\n", file, orig)
		} else {
			sums[s] = file
		}
	}
}

func put(cmd string, args []string) {
	desc := "copies given files to the library path. If no args are given, reads a list of files from stdin."
	fs := newFlagSet("put", "[FILE...]", desc)
	norename := fs.Bool("norename", false, "true to not rename files with an exif date or sha1 hash prefix")
	sum := fs.Bool("sum", true, "true to include a sha1 hash in a notes file")
	thumb := fs.Bool("thumb", true, "true to create and add a thumbnail also")
	fs.Parse(args)

	files := fs.Args()
	if len(files) == 0 {
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

		newname, err := piclib.Add(p, !*norename)
		if piclib.IsDup(err) {
			fmt.Printf("[SKIP] %v\n", err)
		} else if err != nil {
			log.Printf("[ERR] %v\n", err)
		} else {
			fmt.Printf("[ADD] %v\n", p)
			if *thumb {
				err := piclib.MakeThumb(p, 1000, 0)
				if err != nil {
					log.Print("[ERR] %v", err)
				} else {
					fmt.Printf("[THUMB] %v\n", p)
				}
			}
		}

		if *sum {
			if err := piclib.SaveChecksum(newname); err != nil {
				log.Printf("[ERR] %v\n", err)
			}
		}
	}
}

func thumb(cmd string, args []string) {
	desc := "creates thumbnail images for the given files. If no args are given, reads a list of files from stdin."
	fs := newFlagSet("thumb", "[FILE...]", desc)
	w := fs.Uint("w", 1000, "thumb width (px). 0 to preserve aspect ratio based on height.")
	h := fs.Uint("h", 0, "thumb height (px). 0 to preserve aspect ratio based on width.")
	fs.Parse(args)

	files := fs.Args()
	if len(files) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		files = strings.Fields(string(data))
	}

	if *w == 0 && *h == 0 {
		log.Fatal("either width or height must be non-zero")
	}

	for _, path := range files {
		p := strings.TrimSpace(path)
		if p == "" {
			continue
		}

		err := piclib.MakeThumb(p, *w, *h)
		if err != nil {
			log.Print("[ERR] %v", err)
		} else {
			fmt.Printf("[THUMB] %v\n", p)
		}
	}
}

func validate(cmd string, args []string) {
	desc := "verifies checksums of given files. If no args are given, reads a list of files from stdin."
	fs := newFlagSet("validate", "[FILE...]", desc)
	all := fs.Bool("all", false, "true validate every file in the library")
	calc := fs.Bool("calc", false, "true to calculate and store the checksum if it doesn't exist")
	v := fs.Bool("v", false, "verbose output")
	fs.Parse(args)

	files := fs.Args()
	if *all {
		var err error
		files, err = piclib.List(-1)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(files) == 0 {
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

		err := piclib.Validate(p)
		if piclib.IsNoSum(err) {
			if *calc {
				if err := piclib.SaveChecksum(p); err != nil {
					log.Printf("[ERR] %v\n", err)
				} else if *v {
					log.Printf("[VALID] %v\n", p)
				}
			} else {
				log.Printf("[ERR] %v\n", err)
			}
		} else if err != nil {
			log.Printf("[ERR] %v\n", err)
		} else if *v {
			fmt.Printf("[VALID] %v\n", p)
		}
	}
}

func find(cmd string, args []string) {
	desc := "Find and list pictures."
	fs := newFlagSet("find", "", desc)
	fs.Parse(args)

	files, err := piclib.List(-1)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		fmt.Println(filepath.Base(file))
	}
}
