//go:generate go-bindata ./data/...

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kierdavis/dateparser"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/toqueteos/webbrowser"
)

var libpath = flag.String("lib", piclib.DefaultPath(), "path to picture library")

type CmdFunc func(cmd string, args []string)

var cmds = map[string]CmdFunc{
	"add":      add,
	"validate": validate,
	"list":     list,
	"fix":      fix,
	"link":     link,
	"copy":     cpy,
	"serve":    serve,
	"view":     view,
	"note":     note,
}

func newFlagSet(cmd, args, desc string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
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
	check(err)

	cmd, ok := cmds[flag.Arg(0)]
	if !ok {
		flag.Usage()
		return
	}
	cmd(path.Base(flag.Arg(0)), flag.Args()[1:])
}

func add(cmd string, args []string) {
	desc := "copies given files into the library (file names can be piped from stdin)"
	fs := newFlagSet(cmd, "[FILE...]", desc)
	fs.Parse(args)

	files := fs.Args()
	if len(files) == 0 {
		data, err := ioutil.ReadAll(os.Stdin)
		check(err)
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
			log.Printf("[ERROR] %v\n", err)
		} else {
			fmt.Printf("[ADD] %v\n", p.Name)
		}
	}
}

func validate(cmd string, args []string) {
	desc := "verifies checksums of given files (piped from list subcmd is supported)"
	fs := newFlagSet(cmd, "[PIC-ID...]", desc)
	all := fs.Bool("all", false, "true validate every file in the library")
	v := fs.Bool("v", false, "verbose outadd")
	fs.Parse(args)

	var err error
	var pics []*piclib.Pic
	if *all {
		pics, err = lib.List(0, 0)
		check(err)
	} else {
		pics = idsOrStdin(fs.Args())
	}

	for _, p := range pics {
		err := p.Validate()
		if err != nil {
			log.Printf("[ERROR] %v\n", err)
		} else if *v {
			fmt.Printf("[VALID] %v (%v)\n", p.Filepath(), p.Name)
		}
	}
}

func Untracked() []string {
	dir, err := os.Open(*libpath)
	check(err)
	defer dir.Close()

	names, err := dir.Readdirnames(-1)
	check(err)

	pics, err := lib.List(0, 0)
	check(err)

	picmap := map[string]bool{piclib.Libname: true}
	for _, p := range pics {
		picmap[filepath.Base(p.Filepath())] = true
	}

	untracked := []string{}
	for _, name := range names {
		_, ok := picmap[name]
		if !ok {
			untracked = append(untracked, name)
		}
	}
	return untracked
}

func fix(cmd string, args []string) {
	desc := "perform library maintenance"
	fs := newFlagSet(cmd, "", desc)
	untracked := fs.Bool("untracked", false, "print untracked files in the library directory")
	fnames := fs.Bool("fnames", false, "fix miss-named files in the library directory")
	fs.Parse(args)

	if *untracked {
		names := Untracked()
		for _, name := range names {
			fmt.Println(filepath.Join(*libpath, name))
		}
		return
	}

	if *fnames {
		pics, err := lib.List(0, 0)
		check(err)

		names := Untracked()
		files := map[[32]byte]string{}
		var sum [32]byte
		for _, name := range names {
			f, err := os.Open(filepath.Join(*libpath, name))
			check(err)

			sm, err := piclib.Sha256(f)
			check(err)
			copy(sum[:], sm)

			files[sum] = filepath.Join(*libpath, name)
			f.Close()
		}

		for _, p := range pics {
			err := p.Validate()
			if err != nil {
				copy(sum[:], p.Sum)
				fpath, ok := files[sum]
				if ok {
					err := os.Rename(fpath, p.Filepath())
					check(err)
					fmt.Printf("renamed '%v' to '%v'\n", fpath, p.Filepath())
				}
			}
		}
	}
}

type Piclist []*piclib.Pic

func (l Piclist) Len() int           { return len(l) }
func (l Piclist) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l Piclist) Less(i, j int) bool { return l[i].Taken.Before(l[j].Taken) }

func link(cmd string, args []string) {
	desc := "create nicely named sym-links to the identified pics (pipe from list subcmd is supported)"
	fs := newFlagSet(cmd, "[PIC-ID...]", desc)
	dst := fs.String("dst", "./link-pics", "destination directory for sym-links")
	tree := fs.Bool("tree", false, "build a date-tree of the images")
	fs.Parse(args)

	pics := idsOrStdin(fs.Args())

	err := os.MkdirAll(*dst, 0755)
	check(err)

	if *tree {
		prevpath := ""
		sort.Sort(Piclist(pics))
		for _, p := range pics {
			mo := p.Taken.Month().String()
			yr := fmt.Sprint(p.Taken.Year())
			currpath := filepath.Join(*dst, yr, mo)
			if currpath != prevpath {
				err = os.MkdirAll(currpath, 0755)
				check(err)
				prevpath = currpath
			}

			pname := fmt.Sprintf("%v-%v", p.Name, p.Id)
			linkpath := filepath.Join(currpath, pname)
			_, err := os.Stat(linkpath)
			if err == nil {
				log.Fatalf("destination file '%v' exists", linkpath)
			}
			err = os.Symlink(p.Filepath(), linkpath)
			check(err)
		}
	} else {
		for _, p := range pics {
			pname := fmt.Sprintf("%v-%v", p.Name, p.Id)
			linkpath := filepath.Join(*dst, pname)
			_, err := os.Stat(linkpath)
			if err == nil {
				log.Fatalf("destination file '%v' exists", linkpath)
			}
			err = os.Symlink(p.Filepath(), linkpath)
			check(err)
		}
	}
}

func cpy(cmd string, args []string) {
	desc := "copy identified pics out of the library (pipe from list subcmd is supported)"
	fs := newFlagSet(cmd, "[PIC-ID...]", desc)
	dst := fs.String("dst", "./copy-pics", "destination directory for the copies")
	fs.Parse(args)

	pics := idsOrStdin(fs.Args())

	err := os.MkdirAll(*dst, 0755)
	check(err)

	for _, p := range pics {
		ext := filepath.Ext(p.Name)
		pname := p.Name[:len(p.Name)-len(ext)]
		pname = fmt.Sprintf("%v-%v%v", pname, p.Id, ext)
		copypath := filepath.Join(*dst, pname)
		_, err := os.Stat(copypath)
		if err == nil {
			log.Fatalf("destination file '%v' exists", copypath)
		}

		f, err := os.Create(copypath)
		check(err)
		r, err := p.Open()
		check(err)
		_, err = io.Copy(f, r)
		check(err)

		f.Close()
		r.Close()
	}
}

func list(cmd string, args []string) {
	desc := "Find and list pictures."
	fs := newFlagSet(cmd, "", desc)
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

	err = WriteLines(os.Stdout, pics...)
	check(err)
}

func note(cmd string, args []string) {
	desc := "print or modify pictures' notes (piped from list subcmd is supported)"
	fs := newFlagSet(cmd, "", desc)
	replace := fs.Bool("replace", false, "replace notes instead of appending")
	text := fs.String("text", "", "text to append or replace notes")
	fs.Parse(args)

	pics := idsOrStdin(fs.Args())

	if *text == "" && !*replace { // just print notes
		if len(pics) == 1 {
			notes, err := pics[0].GetNotes()
			check(err)
			fmt.Println(notes)
		} else {
			WriteLines(os.Stdout, pics...)
		}
	} else if *replace {
		for _, p := range pics {
			err := p.SetNotes(*text)
			check(err)
		}
	} else { // append text
		for _, p := range pics {
			notes, err := p.GetNotes()
			check(err)
			err = p.SetNotes(notes + "\n" + *text)
			check(err)
		}
	}
}

func serve(cmd string, args []string) {
	desc := "serve listed pics in a browser-based picture gallery (or piped from stdin)"
	fs := newFlagSet(cmd, "[PIC-ID...]", desc)
	fs.StringVar(&addr, "addr", "127.0.0.1:7777", "ip and port to serve gallery at")
	view := fs.Bool("view", false, "opens browser window to gallery page")
	fs.BoolVar(&noedit, "noedit", false, "don't allow editing of anything in library")
	fs.BoolVar(&all, "all", false, "true to view every file in the library")
	fs.Parse(args)

	l, err := net.Listen("tcp", addr)
	check(err)
	go runserve(l, fs.Args())

	if *view {
		err = webbrowser.Open(addr)
		check(err)
	}

	select {}
}

func view(cmd string, args []string) {
	desc := "view listed pictures in browser-based gallery (or piped from stdin)"
	fs := newFlagSet(cmd, "[PIC-ID...]", desc)
	fs.BoolVar(&noedit, "noedit", false, "don't allow editing of anything in library")
	fs.StringVar(&addr, "addr", "127.0.0.1:", "ip and port to serve gallery at")
	fs.BoolVar(&all, "all", false, "true to view every file in the library")
	fs.Parse(args)

	l, err := net.Listen("tcp", addr)
	check(err)
	addr = l.Addr().String()
	go runserve(l, fs.Args())

	err = webbrowser.Open(addr)
	check(err)

	select {}
}
