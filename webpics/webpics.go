package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/rwcarlsen/gallery/piclib"
)

const picsPerPage = 24

var (
	addr    = flag.String("addr", "127.0.0.1:7777", "ip and port to listen on")
	noedit  = flag.Bool("noedit", false, "don't allow editing of anything in library")
	libpath = flag.String("lib", piclib.DefaultPath(), "path to picture library")
	all     = flag.Bool("all", false, "true to view every file in the library")
	dosort  = flag.Bool("sort", true, "true to sort files in reverse chronological order")
)

type Photo struct {
	*piclib.Pic
	Index int
}

var (
	zoomTmpl *template.Template
	gridTmpl *template.Template
	utilTmpl *template.Template
)

var (
	resPath   = os.Getenv("WEBPICS")
	allPhotos = []*Photo{}
	picMap    = map[int]*Photo{}
	contexts  = make(map[string]*context)
	store     = sessions.NewCookieStore([]byte("my-secret"))
	slidepage []byte // slideshow.html
	lib       *piclib.Lib
)

func (p Photo) Date() string {
	return p.Taken.Format("Jan 2, 2006")
}

func (p Photo) Style() string {
	t := fmt.Sprintf("transform:rotate(%vdeg)", p.Orient)
	//Cross-browser
	return fmt.Sprintf("-moz-%s; -webkit-%s; -ms-%s; -o-%s; %s;", t, t, t, t, t)
}

func init() {
	if resPath == "" {
		resPath = "."
	}

	zt := filepath.Join(resPath, "zoompic.html")
	ut := filepath.Join(resPath, "util.html")
	it := filepath.Join(resPath, "index.html")

	zoomTmpl = template.Must(template.ParseFiles(zt, ut))
	gridTmpl = template.Must(template.ParseFiles(it, ut))
	utilTmpl = template.Must(template.ParseFiles(ut))
}

func main() {
	flag.Parse()
	var err error

	lib, err = piclib.Open(*libpath)
	if err != nil {
		log.Fatal(err)
	}

	slidepage, err = ioutil.ReadFile(filepath.Join(resPath, "slideshow.html"))
	if err != nil {
		log.Fatal(err)
	}

	loadPics()
	if *dosort && len(allPhotos) > 0 {
		sort.Sort(newFirst(allPhotos))
	}

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
	log.Printf("listening on %v", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}

var skipext = []string{"", ".avi", ".m4v", ".go"}

func loadPics() {
	var pics []*piclib.Pic
	var err error
	if *all {
		pics, err = lib.List(0, 0)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(flag.Args()) == 0 {
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
		photo := &Photo{Pic: p}
		allPhotos = append(allPhotos, photo)
		picMap[p.Id] = photo
	}
}

type newFirst []*Photo

func (pl newFirst) Less(i, j int) bool {
	itm := pl[i].Taken
	jtm := pl[j].Taken
	return itm.After(jtm)
}
func (pl newFirst) Len() int      { return len(pl) }
func (pl newFirst) Swap(i, j int) { pl[i], pl[j] = pl[j], pl[i] }

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
	http.ServeFile(w, r, filepath.Join(resPath, "static", vars["path"]))
}

func PhotoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		log.Print(err)
		return
	}

	p, ok := picMap[id]
	if !ok {
		log.Print("pic %v not valid", p.Name)
		return
	}

	switch vars["type"] {
	case "orig":
		err = writeImg(w, p.Id, false)
	case "thumb":
		err = writeImg(w, p.Id, true)
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
	if *noedit {
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
