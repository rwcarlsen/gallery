package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/rwcarlsen/gallery/conf"
	"github.com/rwcarlsen/gallery/piclib"
)

const (
	cacheSize   = 300 * piclib.Mb
	picsPerPage = 20
)

var (
	addr   = flag.String("addr", "127.0.0.1:7777", "ip and port to listen on")
	noedit = flag.Bool("noedit", false, "don't allow editing of anything in library")
)

var (
	logger    = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	resPath   = conf.Default.WebpicsAssets()
	lib       *piclib.Library
	allPhotos []*piclib.Photo
	picMap    = map[string]*piclib.Photo{}
	contexts  = make(map[string]*context)
	store     = sessions.NewCookieStore([]byte("my-secret"))
	home      []byte // index.html
	slidepage []byte // slideshow.html
)

func main() {
	flag.Parse()
	var err error
	home, err = ioutil.ReadFile(filepath.Join(resPath, "index.html"))
	if err != nil {
		logger.Fatal(err)
	}

	slidepage, err = ioutil.ReadFile(filepath.Join(resPath, "slideshow.html"))
	if err != nil {
		logger.Fatal(err)
	}

	back, err := conf.Default.Backend()
	if err != nil {
		log.Fatal(err)
	}
	lib, err = piclib.Open(conf.Default.LibName(), back, cacheSize)
	if err != nil {
		log.Fatal(err)
	}
	defer lib.Close()

	loadPics()

	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/static/{path:.*}", StaticHandler)
	r.HandleFunc("/piclib/{imgType}/{picName}", PhotoHandler)
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
	logger.Printf("listening on %v", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		logger.Fatal(err)
	}
}

func loadPics() {
	names := []string{}
	if len(flag.Args()) > 0 {
		names = flag.Args()
	} else {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			logger.Printf("failed to read names from stdin: %v", err)
		}
		names = strings.Fields(string(data))
	}

	for _, name := range names {
		name = path.Base(name)
		p, err := lib.GetPhoto(name)
		if err != nil {
			logger.Printf("err on %v: %v", name, err)
		} else {
			allPhotos = append(allPhotos, p)
			picMap[name] = p
		}
	}
}

///////////////////////////////////////////////////////////
///// static content handlers /////////////////////////////
///////////////////////////////////////////////////////////

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(home)
}

func StaticHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	http.ServeFile(w, r, filepath.Join(resPath, "static", vars["path"]))
}

func PhotoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	data, p, err := fetchImg(vars["imgType"], vars["picName"])
	if err != nil {
		logger.Print(err)
		return
	} else if !strings.Contains(p.Tags[noteField], *filter) {
		logger.Printf("Unauthorized access attempt to pic %v", vars["picName"])
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	disp := "attachment; filename=\"" + p.Orig + "\""
	w.Header().Set("Content-Disposition", disp)
	w.Write(data)
}

const (
	MetaFile  = "meta"
	OrigImg   = "orig"
	Thumb1Img = "thumb1"
	Thumb2Img = "thumb2"
)

func fetchImg(imgType, picName string) ([]byte, *piclib.Photo, error) {
	p, ok := picMap[picName]
	if !ok {
		return nil, nil, fmt.Errorf("picname %v not valid", picName)
	}

	var data []byte
	var err error
	switch imgType {
	case MetaFile:
		data, err = json.Marshal(p)
	case OrigImg:
		data, err = p.GetOriginal()
	case Thumb1Img:
		data, err = p.GetThumb1()
	case Thumb2Img:
		data, err = p.GetThumb2()
	default:
		return nil, nil, fmt.Errorf("invalid image type '%v'", imgType)
	}

	if err != nil {
		return nil, nil, err
	}
	return data, p, nil
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
			logger.Print(err)
		}
	}
}

func NotesHandler(w http.ResponseWriter, r *http.Request) {
	if *noedit {
		return
	}
	c, vars := getContext(w, r)
	if err := c.saveNotes(r, vars["picIndex"]); err != nil {
		logger.Print(err)
	}
}

func NextSlideHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	if err := c.serveSlide(w); err != nil {
		logger.Print(err)
	}
}

func SlideStyleHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	c.initRand()
	p := c.photos[c.random[c.randIndex]]
	w.Write([]byte(imgRotJS(p.Rotation())))
}

func SlideshowHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(slidepage)
}

func ZoomHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	if err := c.serveZoom(w, vars["index"]); err != nil {
		logger.Print(err)
	}
}

func PageNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	if err := c.servePageNav(w); err != nil {
		logger.Print(err)
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
		logger.Print(err)
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
		contexts[v.(string)] = newContext(allPhotos, *filter)
	} else if _, ok := contexts[v.(string)]; !ok {
		v = time.Now().String()
		s.Values["context-id"] = v
		contexts[v.(string)] = newContext(allPhotos, *filter)
	}
	s.Save(r, w)
	c := contexts[v.(string)]

	vars := mux.Vars(r)
	return c, vars
}
