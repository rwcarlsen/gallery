package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
	"flag"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/rwcarlsen/gallery/backend"
	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goamz/aws"
)

const (
	libName     = "rwc-piclib"
	cacheSize   = 300 * piclib.Mb
	picsPerPage = 20
)

var (
	addr        = flag.String("addr", "127.0.0.1:7777", "ip and port to listen on")
	resPath   = os.Getenv("WEBPICS")
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
		log.Fatal(err)
	}

	slidepage, err = ioutil.ReadFile(filepath.Join(resPath, "slideshow.html"))
	if err != nil {
		log.Fatal(err)
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	db := localBackend()
	lib = piclib.New(libName, db, cacheSize)
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
	log.Printf("listening on %v", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}

func localBackend() backend.Interface {
	return &localhd.Backend{Root: "/home/robert/Pictures"}
}

func amzBackend() backend.Interface {
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	return amz.New(auth, aws.USEast)
}

func updateLib() {
	names, err := lib.ListNames(20000)
	if err != nil {
		log.Println(err)
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
						log.Printf("err on %v: %v", name, err)
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
		log.Println(err)
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
				log.Println(err)
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
	log.Println("done uploading")

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
		log.Print(err)
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
		log.Println("pName: ", picName)
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
		c.servePage(w, pg)
	}
}

func NotesHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	c.saveNotes(r, vars["picIndex"])
}

func NextSlideHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	c.serveSlide(w)
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
	vars := mux.Vars(r)

	p, err := lib.GetPhoto(vars["pic"])
	if err != nil {
		log.Print(err)
		return
	}

	p.Tags[noteField] += string("\n" + vars["tag"])
	if err := lib.UpdatePhoto(p); err != nil {
		log.Print(err)
	}
}

func ZoomHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	c.serveZoom(w, vars["index"])
}

func PageNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	c.servePageNav(w)
}

func StatHandler(w http.ResponseWriter, r *http.Request) {
	c, vars := getContext(w, r)
	c.serveStat(w, vars["stat"])
}

func TimeNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	c.serveTimeNav(w)
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
		contexts[v.(string)] = &context{photos: allPhotos, CurrPage: "1"}
	} else if _, ok := contexts[v.(string)]; !ok {
		v = time.Now().String()
		s.Values["context-id"] = v
		contexts[v.(string)] = &context{photos: allPhotos, CurrPage: "1"}
	}
	s.Save(r, w)
	c := contexts[v.(string)]

	vars := mux.Vars(r)
	return c, vars
}

