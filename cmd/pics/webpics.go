package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/rwcarlsen/gallery/piclib"
)

const picsPerPage = 24

// rots holds mappings from exif orientation tag to degrees clockwise needed
var rots = map[int]int{
	1: 0,
	2: 0,
	3: 180,
	4: 180,
	5: 90,
	6: 90,
	7: 270,
	8: 270,
}

var (
	zoomTmpl *template.Template
	gridTmpl *template.Template
	utilTmpl *template.Template
)

var (
	allPhotos = []*Photo{}
	picMap    = map[int]*Photo{}
	contexts  = make(map[string]*context)
	store     = sessions.NewCookieStore([]byte("my-secret"))
	slidepage []byte // slideshow.html
)

type Photo struct {
	*piclib.Pic
	Index int
}

func (p Photo) Date() string {
	return p.Taken.Format("Jan 2, 2006")
}

func (p Photo) Style() string {
	t := fmt.Sprintf("transform:rotate(%vdeg)", rots[p.Orient])
	//Cross-browser
	return fmt.Sprintf("-moz-%s; -webkit-%s; -ms-%s; -o-%s; %s;", t, t, t, t, t)
}

func init() {
	zt, err := Asset("data/zoompic.html")
	if err != nil {
		log.Fatal(err)
	}
	ut, err := Asset("data/util.html")
	if err != nil {
		log.Fatal(err)
	}
	it, err := Asset("data/index.html")
	if err != nil {
		log.Fatal(err)
	}

	zoomTmpl = template.Must(template.New("zoompic").Parse(string(append(zt, ut...))))
	gridTmpl = template.Must(template.New("index").Parse(string(append(it, ut...))))
	utilTmpl = template.Must(template.New("util").Parse(string(ut)))
}

var (
	noedit bool
	addr   string
	all    bool
)

func serve(cmd string, args []string) {
	desc := "Run a browser-based picture gallery of listed pics (or piped from stdin)"
	fs := newFlagSet(cmd, "[PIC-ID...]", desc)
	fs.StringVar(&addr, "addr", "127.0.0.1:7777", "ip and port to listen on")
	fs.BoolVar(&noedit, "noedit", false, "don't allow editing of anything in library")
	fs.BoolVar(&all, "all", false, "true to view every file in the library")
	fs.Parse(args)

	var err error
	slidepage, err = Asset("data/slideshow.html")
	if err != nil {
		log.Fatal(err)
	}

	loadPics()

	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/static/{path:.*}", StaticHandler)
	r.HandleFunc("/photo/{type}/{id}", PhotoHandler)
	r.HandleFunc("/dynamic/pg{pg:[0-9]*}", PageHandler)
	r.HandleFunc("/dynamic/zoom/{index:[0-9]+}", ZoomHandler)
	r.HandleFunc("/dynamic/page-nav", PageNavHandler)
	r.HandleFunc("/dynamic/time-nav", TimeNavHandler)
	r.HandleFunc("/dynamic/set-page/{page:[0-9]+}", SetPageHandler)
	r.HandleFunc("/dynamic/stat/{stat}", StatHandler)
	r.HandleFunc("/dynamic/save-notes/{picIndex:[0-9]+}", NotesHandler)
	r.HandleFunc("/dynamic/slideshow", SlideshowHandler)
	r.HandleFunc("/dynamic/next-slide", NextSlideHandler)
	r.HandleFunc("/dynamic/slide-style", SlideStyleHandler)

	http.Handle("/", r)
	log.Printf("listening on %v", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

var skipext = []string{"", ".avi", ".m4v", ".go"}

func loadPics() {
	var pics []*piclib.Pic
	var err error
	if all {
		pics, err = lib.List(0, 0)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(flag.Args()) == 0 {
		pics, err = piclib.LoadStream(lib, os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		for _, idstr := range flag.Args() {
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
		if p.Ext() == ".avi" || p.Ext() == ".m4v" {
			continue
		}
		photo := &Photo{Pic: p}
		allPhotos = append(allPhotos, photo)
		picMap[p.Id] = photo
	}
}

///////////////////////////////////////////////////////////
///// static content handlers /////////////////////////////
///////////////////////////////////////////////////////////

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if err := gridTmpl.Execute(w, nil); err != nil {
		log.Print(err)
	}
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	data, err := Asset(path.Join("data/static", vars["path"]))
	if err != nil {
		log.Print(err)
		return
	}
	switch ext := path.Ext(vars["path"]); ext {
	case ".js":
		w.Header().Set("Content-Type", "text/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	}
	w.Write(data)
}

func PhotoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		log.Print(err)
		return
	}

	switch vars["type"] {
	case "orig":
		err = writeImg(w, id, false)
	case "thumb":
		err = writeImg(w, id, true)
	default:
		log.Print("invalid pic type %v", vars["type"])
		return
	}

	if err != nil {
		log.Print(err)
	}
}

func writeImg(w io.Writer, id int, thumb bool) error {
	p, ok := picMap[id]
	if !ok {
		return fmt.Errorf("%v is not a valid pic id", id)
	}

	if thumb {
		data, err := p.Thumb()
		if err != nil {
			return err
		}
		w.Write(data)
		return nil
	}

	r, err := p.Open()
	if err != nil {
		return err
	}
	defer r.Close()

	_, err = io.Copy(w, r)
	return err
}

///////////////////////////////////////////////////////////
///// dynamic content (context-specific) handlers /////////
///////////////////////////////////////////////////////////

func PageHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	if pg := vars["pg"]; len(pg) == 0 {
		fmt.Fprint(w, c.CurrPage)
	} else {
		if err := c.servePage(w, pg); err != nil {
			log.Print(err)
		}
	}
}

func NotesHandler(w http.ResponseWriter, r *http.Request) {
	if noedit {
		return
	}
	c, vars := getContext(w, r)
	if err := c.saveNotes(r, vars["picIndex"]); err != nil {
		log.Print(err)
	}
}

func NextSlideHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	if err := c.serveSlide(w); err != nil {
		log.Print(err)
	}
}

func SlideStyleHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	c.initRand()
	p := c.photos[c.random[c.randIndex]]
	w.Write([]byte(p.Style()))
}

func SlideshowHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(slidepage)
}

func ZoomHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	if err := c.serveZoom(w, vars["index"]); err != nil {
		log.Print(err)
	}
}

func PageNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	if err := c.servePageNav(w); err != nil {
		log.Print(err)
	}
}

func StatHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	c.serveStat(w, vars["stat"])
}

func SetPageHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	c.CurrPage = vars["page"]
}

func TimeNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	if err := c.serveTimeNav(w); err != nil {
		log.Print(err)
	}
}

func getContext(w http.ResponseWriter, r *http.Request) (*context, map[string]string) {
	s, err := store.Get(r, "dyn-content")
	if err != nil {
		panic(err.Error())
	}

	v, ok := s.Values["context-id"]
	if !ok {
		v = time.Now().String()
		s.Values["context-id"] = v
		contexts[v.(string)] = newContext(allPhotos)
	} else if _, ok := contexts[v.(string)]; !ok {
		v = time.Now().String()
		s.Values["context-id"] = v
		contexts[v.(string)] = newContext(allPhotos)
	}
	s.Save(r, w)
	c := contexts[v.(string)]

	vars := mux.Vars(r)
	return c, vars
}
