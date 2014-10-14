// picput recursively walks passed dirs and photos and adds them to a library.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kierdavis/dateparser"
	"github.com/rwcarlsen/gallery/piclib"
)

var libpath = flag.String("lib", piclib.DefaultPath(), "path to picture library")

type CmdFunc func(cmd string, args []string)

var cmds = map[string]CmdFunc{
	"put":      put,
	"validate": validate,
	"list":     list,
}

func newFlagSet(cmd, args, desc string) *flag.FlagSet {
	fs := flag.NewFlagSet("put", flag.ExitOnError)
	fs.Usage = func() {
		log.Printf("Usage: pics %s [OPTION] %s\n%s\n", cmd, args, desc)
		fs.PrintDefaults()
	}
	return fs
}

var lib *piclib.Lib

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

	if len(flag.Args()) == 0 {
		flag.Usage()
		return
	}

	var err error
	lib, err = piclib.Open(*libpath)
	if err != nil {
		log.Fatal(err)
	}

	cmd, ok := cmds[flag.Arg(0)]
	if !ok {
		flag.Usage()
		return
	}
	cmd(flag.Arg(0), flag.Args()[1:])
}

func put(cmd string, args []string) {
	desc := "copies given files into the library. Uses given args or reads a list of files from stdin."
	fs := newFlagSet("put", "[FILE...]", desc)
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
		path := strings.TrimSpace(path)
		if path == "" {
			continue
		}

		p, err := lib.AddFile(path)
		if piclib.IsDup(err) {
			fmt.Printf("[SKIP] %v\n", err)
		} else if err != nil {
			log.Printf("[ERR] %v\n", err)
		} else {
			fmt.Printf("[ADD] %v\n", p.Name)
		}
	}
}

func validate(cmd string, args []string) {
	desc := "verifies checksums of given files. If no args are given, reads a list of files from stdin."
	fs := newFlagSet("validate", "[FILE...]", desc)
	all := fs.Bool("all", false, "true validate every file in the library")
	v := fs.Bool("v", false, "verbose output")
	fs.Parse(args)

	var err error
	var pics []*piclib.Pic
	if *all {
		pics, err = lib.List(0, 0)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(fs.Args()) == 0 {
		dec := json.NewDecoder(bufio.NewReader(os.Stdin))
		for {
			p := &piclib.Pic{}
			err = dec.Decode(&p)
			if err != nil {
				break
			}
			preal, err := lib.Open(p.Id)
			if err != nil {
				log.Fatal(err)
			}
			pics = append(pics, preal)
		}
		if err != io.EOF {
			log.Fatal(err)
		}
	} else {
		for _, idstr := range fs.Args() {
			id, err := strconv.Atoi(idstr)
			if err != nil {
				log.Fatal(err)
			}
			p, err := lib.Open(id)
			if err != nil {
				log.Fatal(err)
			}
			pics = append(pics, p)
		}
	}

	for _, p := range pics {
		err := p.Validate()
		if err != nil {
			log.Printf("[ERR] %v\n", err)
		} else if *v {
			fmt.Printf("[VALID] %v\n", p)
		}
	}
}

type picmeta struct {
	Id    int
	Taken time.Time
	Notes string
}

func list(cmd string, args []string) {
	desc := "Find and list pictures."
	fs := newFlagSet("list", "", desc)
	after := fs.String("from", "", "only show photos after date")
	before := fs.String("to", "", "only show photos before date")
	fs.Parse(args)

	var err error
	at := time.Time{}
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

	pics, err := lib.ListTime(at, bt)
	if err != nil {
		log.Fatal(err)
	}

	for _, p := range pics {
		notes, err := p.GetNotes()
		if err != nil {
			log.Fatal(err)
		}
		m := &picmeta{
			Id:    p.Id,
			Taken: p.Taken,
			Notes: notes,
		}
		data, err := json.Marshal(m)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(data))
	}
}
