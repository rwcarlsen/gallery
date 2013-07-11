package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/piclib"
)

const (
	libName     = "rwc-piclib"
	cacheSize   = 300 * piclib.Mb
	picsPerPage = 20
)

var (
	addr        = flag.String("addr", "127.0.0.1:7777", "ip and port to listen on")
	db          = flag.String("db", "hd", "name of backend  described in conf file")
	filter      = flag.String("filter", "", "only serve pics with notes that match filter text")
	disableEdit = flag.Bool("noedit", false, "don't allow editing of anything in library")
)

var (
	logger    = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	resPath   = os.Getenv("WEBPICS")
	confPath  = filepath.Join(os.Getenv("HOME"), ".backends")
	lib       *piclib.Library
	allPhotos []*piclib.Photo
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

	back := makeBackend()
	lib, err = piclib.Open(libName, back, cacheSize)
	if err != nil {
		log.Fatal(err)
	}
	defer lib.Close()

	updateLib()

	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/static/{path:.*}", StaticHandler)
	r.HandleFunc("/addphotos", AddPhotoHandler)
	r.HandleFunc("/piclib/{imgType}/{picName}", PhotoHandler)
	r.HandleFunc("/tagit/{tag}/{pic}", TagHandler)
	r.HandleFunc("/dynamic/pg{pg:[0-9]*}", PageHandler)
	r.HandleFunc("/dynamic/zoom/{index:[0-9]+}", ZoomHandler)
	r.HandleFunc("/dynamic/page-nav", PageNavHandler)
	r.HandleFunc("/dynamic/time-nav", TimeNavHandler)
	r.HandleFunc("/dynamic/toggle-dateless", DateToggleHandler)
	r.HandleFunc("/dynamic/stat/{stat}", StatHandler)
	r.HandleFunc("/dynamic/save-notes/{picIndex:[0-9]+}", NotesHandler)
	r.HandleFunc("/dynamic/slideshow", SlideshowHandler)
	r.HandleFunc("/dynamic/next-slide", NextSlideHandler)
	r.HandleFunc("/dynamic/slide-style", SlideStyleHandler)
	r.HandleFunc("/dynamic/search-query", SearchHandler)

	http.Handle("/", r)
	logger.Printf("listening on %v", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		logger.Fatal(err)
	}
}

func makeBackend() backend.Interface {
	f, err := os.Open(confPath)
	if err != nil {
		log.Fatal(err)
	}
	set, err := backend.LoadSpecList(f)
	if err != nil {
		log.Fatal(err)
	}

	back, err := set.Make(*db)
	if err != nil {
		log.Fatal(err)
	}
	return back
}

func updateLib() {
	names, err := lib.ListNames(20000)
	if err != nil {
		logger.Println(err)
	}

	nWorkers := 10
	picCh := make(chan *piclib.Photo)
	nameCh := make(chan string)
	done := make(chan bool)
	for i := 0; i < nWorkers; i++ {
		go func() {
			for {
				select {
				case name := <-nameCh:
					p, err := lib.GetPhoto(name)
					if err != nil {
						logger.Printf("err on %v: %v", name, err)
					}
					picCh <- p
				case <-done:
					return
				}
			}
		}()
	}

	go func() {
		for _, name := range names {
			nameCh <- name
		}
	}()

	for _ = range names {
		if p := <-picCh; p != nil {
			allPhotos = append(allPhotos, p)
		}
	}

	for i := 0; i < nWorkers; i++ {
		done <- true
	}

	if len(allPhotos) > 0 {
		sort.Sort(newFirst(allPhotos))
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

func AddPhotoHandler(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		logger.Println(err)
		return
	} else if *disableEdit {
		return
	}

	picCh := make(chan *piclib.Photo)
	respCh := make(chan map[string]interface{})

	var part *multipart.Part
	count := 0
	for {
		if part, err = mr.NextPart(); err != nil {
			break
		}
		if part.FormName() == "" {
			continue
		} else if part.FileName() == "" {
			continue
		}

		name := part.FileName()
		data, err := ioutil.ReadAll(part)
		resp := map[string]interface{}{
			"name": name,
			"size": len(data),
		}

		count++
		go func(data []byte, nm string, respMeta map[string]interface{}) {
			var p *piclib.Photo
			if err != nil {
				logger.Println(err)
				respMeta["error"] = err.Error()
			} else {
				p, err = lib.AddPhoto(nm, bytes.NewReader(data))
				if err != nil {
					respMeta["error"] = err.Error()
				}
			}
			respCh <- respMeta
			picCh <- p
		}(data, name, resp)
	}

	resps := []interface{}{}
	newPics := []*piclib.Photo{}
	for i := 0; i < count; i++ {
		resp := <-respCh
		p := <-picCh
		resps = append(resps, resp)
		if p != nil {
			newPics = append(newPics, p)
			allPhotos = append(allPhotos, p)
		}
	}
	logger.Println("done uploading")

	sort.Sort(newFirst(allPhotos))
	data, _ := json.Marshal(resps)
	w.Write(data)
	for _, c := range contexts {
		c.addPics(newPics)
	}
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
	p, err := lib.GetPhoto(picName)
	if err != nil {
		logger.Println("pName: ", picName)
		return nil, nil, err
	}

	var data []byte
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
	if *disableEdit {
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

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	r.ParseForm()
	c.setSearchFilter(r.Form["search-query"])
}

func TagHandler(w http.ResponseWriter, r *http.Request) {
	if *disableEdit {
		return
	}
	vars := mux.Vars(r)

	p, err := lib.GetPhoto(vars["pic"])
	if err != nil {
		logger.Print(err)
		return
	}

	p.Tags[noteField] += string("\n" + vars["tag"])
	if err := lib.UpdatePhoto(p); err != nil {
		logger.Print(err)
	}
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

func TimeNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	if err := c.serveTimeNav(w); err != nil {
		logger.Print(err)
	}
}

func DateToggleHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	c.toggleDateless()
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
