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
	"time"

	"github.com/kierdavis/dateparser"
	"github.com/rwcarlsen/gallery/piclib"
)

var lib = flag.String("lib", "", "path to picture library (blank => env PICLIB => $HOME/piclib)")

type CmdFunc func(cmd string, args []string)

var cmds = map[string]CmdFunc{
	"put":      put,
	"fix":      fix,
	"validate": validate,
	"dups":     dups,
	"find":     find,
	"thumb":    thumb,
}

func newFlagSet(cmd, args, desc string) *flag.FlagSet {
	fs := flag.NewFlagSet("put", flag.ExitOnError)
	fs.Usage = func() {
		log.Printf("Usage: pics %s [OPTION] %s\n%s\n", cmd, args, desc)
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
	quiet := fs.Bool("quiet", false, "true to only print file names")
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
			if *quiet {
				fmt.Println(file)
			} else {
				fmt.Printf("[DUP] %v is duplicate of %v\n", file, orig)
			}
		} else {
			sums[s] = file
		}
	}
}

func fix(cmd string, args []string) {
	desc := "fix things"
	fs := newFlagSet("fix", "[FILE...]", desc)
	all := fs.Bool("all", false, "true to check every file in the library")
	sum := fs.Bool("sum", false, "update the sum to actual value (warning - this can be dangerous)")
	date := fs.Bool("date", false, "fix the cached date in the notes file to match EXIF data")
	thumb := fs.Bool("thumb", false, "create and add a thumbnail")
	fs.Parse(args)

	files := fs.Args()
	if *all {
		list, err := piclib.List(-1, "", ".go")
		if err != nil {
			log.Printf("[ERR] %v\n", err)
			return
		}
		files = append(files, list...)
	} else if len(files) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		files = strings.Fields(string(data))
	}

	for _, pic := range files {
		if *date {
			if err := piclib.SaveDate(pic); err != nil {
				log.Printf("[ERR] %v\n", err)
			}
		}
		if *thumb {
			err := piclib.MakeThumb(pic, 1000, 0)
			if err != nil && !piclib.IsDupThumb(err) {
				log.Printf("[ERR] %v", err)
			}
		}
		if *sum {
			if err := piclib.SaveChecksum(pic); err != nil {
				log.Printf("[ERR] %v\n", err)
			}
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
			continue
		} else {
			fmt.Printf("[ADD] %v\n", p)
			if *thumb {
				err := piclib.MakeThumb(newname, 1000, 0)
				if err != nil && !piclib.IsDupThumb(err) {
					log.Printf("[ERR] %v", err)
				}
			}

			if *sum {
				if err := piclib.SaveChecksum(newname); err != nil {
					log.Printf("[ERR] %v\n", err)
				}
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
	after := fs.String("from", "", "only show photos after date")
	before := fs.String("to", "", "only show photos before date")
	fs.Parse(args)

	var err error
	var at time.Time
	bt := time.Now()
	if *after != "" {
		reftime := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		pars := &dateparser.Parser{Default: reftime}
		at, err = pars.Parse(*after)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *before != "" {
		reftime := time.Date(time.Now().Year(), 12, 31, 23, 59, 59, 0, time.UTC)
		pars := &dateparser.Parser{Default: reftime}
		bt, err = pars.Parse(*before)
		if err != nil {
			log.Fatal(err)
		}
	}

	files, err := piclib.List(-1)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		t := piclib.Taken(file)
		if (*after == "" || t.After(at)) && (*before == "" || t.Before(bt)) {
			fmt.Println(filepath.Base(file))
		}
	}
}
