package main

import (
	"encoding/json"
	"fmt"
	"time"
	"html/template"
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
	"launchpad.net/goamz/aws"
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

var indexTmpl = template.Must(template.ParseFiles("index.tmpl"))

var (
	lib    *piclib.Library
	allPhotos []*piclib.Photo
	contexts = make(map[string]*context)
	store = sessions.NewCookieStore([]byte("my-secret"))
)

func main() {
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
	r.HandleFunc("/dynamic/zoom/{index:[0-9]*}", ZoomHandler)
	r.HandleFunc("/dynamic/page-nav", PageNavHandler)
	r.HandleFunc("/dynamic/time-nav", TimeNavHandler)
	r.HandleFunc("/dynamic/toggle-dateless", DateToggleHandler)
	r.HandleFunc("/dynamic/stat/{stat}", StatHandler)

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
	names, err := lib.ListPhotosN(20000)
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
	if err := indexTmpl.Execute(w, nil); err != nil {
		log.Print(err)
	}
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
	for i := 0; i < count; i++ {
		resp := <-respCh
		p := <-picCh
		resps = append(resps, resp)
		if p != nil {
			allPhotos = append(allPhotos, p)
		}
	}
	log.Println("done uploading")

	sort.Sort(newFirst(allPhotos))
	data, _ := json.Marshal(resps)
	w.Write(data)
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
		data, err = lib.GetOriginal(p)
	case Thumb1Img:
		data, err = lib.GetThumb1(p)
	case Thumb2Img:
		data, err = lib.GetThumb2(p)
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
	c, vars, s := getContext(w, r)
	c.servePage(w, vars["pg"])
	if err := s.Save(r, w); err != nil {
		log.Print(err)
	}
}

func ZoomHandler(w http.ResponseWriter, r *http.Request) {
	c, vars, s := getContext(w, r)
	c.serveZoom(w, vars["index"])
	if err := s.Save(r, w); err != nil {
		log.Print(err)
	}
}

func PageNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _, s := getContext(w, r)
	c.servePageNav(w)
	if err := s.Save(r, w); err != nil {
		log.Print(err)
	}
}

func StatHandler(w http.ResponseWriter, r *http.Request) {
	c, vars, s := getContext(w, r)
	c.serveStat(w, vars["stat"])
	if err := s.Save(r, w); err != nil {
		log.Print(err)
	}
}

func TimeNavHandler(w http.ResponseWriter, r *http.Request) {
	c, _, s := getContext(w, r)
	c.serveTimeNav(w)
	if err := s.Save(r, w); err != nil {
		log.Print(err)
	}
}

func DateToggleHandler(w http.ResponseWriter, r *http.Request) {
	c, _, s := getContext(w, r)
	c.toggleDateless()
	if err := s.Save(r, w); err != nil {
		log.Print(err)
	}
}

func getContext(w http.ResponseWriter, r *http.Request) (*context, map[string]string, *sessions.Session) {
	s, _ := store.Get(r, "dyn-content")

	v, ok := s.Values["context-id"]
	if !ok {
		v = time.Now().String()
		s.Values["context-id"] = v
		contexts[v.(string)] = &context{photos: allPhotos}
	}
	c, ok := contexts[v.(string)]
	if !ok {
		panic("failed to find context")
	}

	vars := mux.Vars(r)
	return c, vars, s
}

