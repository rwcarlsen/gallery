package main

import (
	"encoding/json"
	"fmt"
	"time"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"sort"
	"bytes"
	"path/filepath"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/goamz/aws"
	"github.com/gorilla/sessions"
	"github.com/gorilla/mux"
)

const (
	libName = "rwc-piclib"
	cacheSize = 300 * piclib.Mb
	picsPerPage = 28
	addr    = "0.0.0.0:7777"
)

const (
	MetaFile  = "meta"
	OrigImg   = "orig"
	Thumb1Img = "thumb1"
	Thumb2Img = "thumb2"
)

var (
	lib    *piclib.Library
	allPhotos []*piclib.Photo
	contexts = make(map[string]*context)
	store = sessions.NewCookieStore([]byte("my-secret"))
	home []byte // index.html
)

func main() {
	var err error
	home, err = ioutil.ReadFile("index.html")
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
	r.HandleFunc("/dynamic/pg{pg:[0-9]*}", PageHandler)
	r.HandleFunc("/dynamic/zoom/{index:[0-9]+}", ZoomHandler)
	r.HandleFunc("/dynamic/page-nav", PageNavHandler)
	r.HandleFunc("/dynamic/time-nav", TimeNavHandler)
	r.HandleFunc("/dynamic/toggle-dateless", DateToggleHandler)
	r.HandleFunc("/dynamic/stat/{stat}", StatHandler)
	r.HandleFunc("/dynamic/save-notes/{picIndex:[0-9]+}", NotesHandler)
	r.HandleFunc("/dynamic/slideshow", SlideshowHandler)

	http.Handle("/", r)
	log.Printf("listening on %v", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func localBackend() piclib.Backend {
	return &localhd.Backend{Root: "/media/spare"}
}

func amzBackend() piclib.Backend {
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
				case name := <- nameCh:
					p, err := lib.GetPhoto(name)
					if err != nil {
						log.Printf("err on %v: %v", name, err)
					}
					picCh <- p
				case <- done:
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
		if p := <- picCh; p != nil {
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
	http.ServeFile(w, r, filepath.Join("static", vars["path"]))
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
	data, err := fetchImg(vars["imgType"], vars["picName"])
	if err != nil {
		log.Print(err)
		return
	}
	w.Write(data)
}

func fetchImg(imgType, picName string) ([]byte, error) {
	p, err := lib.GetPhoto(picName)
	if err != nil {
		log.Println("pName: ", picName)
		return nil, err
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
		return nil, fmt.Errorf("invalid image type '%v'", imgType)
	}

	if err != nil {
		return nil, err
	}
	return data, nil
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

func SlideshowHandler(w http.ResponseWriter, r *http.Request) {
	c, _ := getContext(w, r)
	c.serveRandom(w)
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

