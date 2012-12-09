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
	"strings"
	"bytes"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/backend/localhd"
	"github.com/rwcarlsen/gallery/piclib"
	"launchpad.net/goamz/aws"
	"github.com/gorilla/sessions"
)


const (
	MetaFile  = "meta"
	OrigImg   = "orig"
	Thumb1Img = "thumb1"
	Thumb2Img = "thumb2"
)

const (
	libName = "rwc-piclib"
	addr    = "0.0.0.0:7777"
)

const (
	picsPerPage = 28
)

var indexTmpl = template.Must(template.ParseFiles("index.tmpl"))
var zoomTmpl = template.Must(template.ParseFiles("templates/zoompic.tmpl"))
var picsTmpl = template.Must(template.ParseFiles("templates/browsepics.tmpl"))
var pagenavTmpl = template.Must(template.ParseFiles("templates/pagination.tmpl"))
var timenavTmpl = template.Must(template.ParseFiles("templates/timenav.tmpl"))

var store = sessions.NewCookieStore([]byte("my-secret"))

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

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	db := localBackend()
	lib := piclib.New(libName, db)

	h := &handler{
		lib: lib,
		contexts: make(map[string]*context),
	}
	h.updateLib()

	http.Handle("/", h)
	log.Printf("listening on %v", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

type newFirst []*piclib.Photo

func (pl newFirst) Less(i, j int) bool {
	itm := pl[i].Taken
	jtm := pl[j].Taken
	return itm.After(jtm)
}

func (pl newFirst) Len() int {
	return len(pl)
}

func (pl newFirst) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}

type handler struct {
	lib    *piclib.Library
	photos []*piclib.Photo
	contexts map[string]*context
}

type year struct {
	Year      int
	StartPage int
	Months    []*month
}

func (y *year) reverseMonths() {
	end := len(y.Months) - 1
	for i := 0; i < len(y.Months)/2; i++ {
		y.Months[i], y.Months[end-i] = y.Months[end-i], y.Months[i]
	}
}

type month struct {
	Name string
	Page int
}

type thumbData struct {
	Path  string
	Date  string
	Index int
}

func (h *handler) updateLib() {
	names, err := h.lib.ListPhotosN(20000)
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
					p, err := h.lib.GetPhoto(name)
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
			h.photos = append(h.photos, p)
		}
	}

	for i := 0; i < nWorkers; i++ {
		done <- true
	}

	if len(h.photos) > 0 {
		sort.Sort(newFirst(h.photos))
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		h.serveHome(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/dynamic") {
		h.serveDynamic(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/static") {
		http.ServeFile(w, r, r.URL.Path[1:])
	} else if strings.HasPrefix(r.URL.Path, "/addphotos") {
		h.serveAddPhotos(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/piclib") {
		h.servePhoto(w, r)
	} else {
		msg := fmt.Sprintf("Invalid request path '%v'", r.URL.Path)
		log.Print(msg)
		http.Error(w, msg, http.StatusNotFound)
	}
}

func (h *handler) serveHome(w http.ResponseWriter, r *http.Request) {
	if err := indexTmpl.Execute(w, nil); err != nil {
		log.Fatal(err)
	}
}

func (h *handler) serveDynamic(w http.ResponseWriter, r *http.Request) {
	items := strings.Split(r.URL.Path, "/")
	if len(items) < 3 {
		log.Printf("invalid dynamic content request path %v", r.URL.Path)
		return
	}

	session, err := store.Get(r, "dyn-content")
	if err != nil {
		log.Printf("failed session retrieval/creation: %v", err)
		return
	}

	v, ok := session.Values["name"]
	if !ok {
		v = time.Now().String()
		session.Values["name"] = v
		h.contexts[v.(string)] = &context{h: h, photos: h.photos}
	}
	c, ok := h.contexts[v.(string)]
	if !ok {
		log.Println("failed to find context")
		return
	}

	switch {
	case strings.HasPrefix(r.URL.Path, "/dynamic/pg"):
		c.servePage(w, r)
	case strings.HasPrefix(r.URL.Path, "/dynamic/zoom"):
		c.serveZoom(w, r)
	case r.URL.Path == "/dynamic/page-nav":
		c.servePageNav(w, r)
	case r.URL.Path == "/dynamic/num-pages":
		c.serveNumPages(w, r)
	case r.URL.Path == "/dynamic/num-pics":
		c.serveNumPics(w, r)
	case r.URL.Path == "/dynamic/time-nav":
		c.serveTimeNav(w, r)
	case r.URL.Path == "/dynamic/toggle-dateless":
		c.toggleDateless()
	case r.URL.Path == "/dynamic/hiding-dateless":
		fmt.Fprint(w, c.HideDateless)
	default:
		log.Printf("invalid dynamic content request path %v", r.URL.Path)
	}

	if err := session.Save(r, w); err != nil {
		log.Println(err)
	}
}

func (h *handler) serveAddPhotos(w http.ResponseWriter, r *http.Request) {
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
				p, err = h.lib.AddPhoto(nm, bytes.NewReader(data))
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
			h.photos = append(h.photos, p)
		}
	}
	log.Println("done uploading")

	sort.Sort(newFirst(h.photos))
	data, _ := json.Marshal(resps)
	w.Write(data)
}

func (h *handler) servePhoto(w http.ResponseWriter, r *http.Request) {
	data, err := h.fetchImg(r.URL.Path)
	if err != nil {
		log.Print(err)
		return
	}
	w.Write(data)
}

func (h *handler) fetchImg(path string) ([]byte, error) {
	items := strings.Split(path[1:], "/")
	if len(items) != 3 {
		return nil, fmt.Errorf("invalid piclib resource '%v'", path)
	}
	imgType, pName := items[1], items[2]

	p, err := h.lib.GetPhoto(pName)
	if err != nil {
		log.Println("pName: ", pName)
		return nil, err
	}

	var data []byte
	switch imgType {
	case MetaFile:
		data, err = json.Marshal(p)
	case OrigImg:
		data, err = h.lib.GetOriginal(p)
	case Thumb1Img:
		data, err = h.lib.GetThumb1(p)
	case Thumb2Img:
		data, err = h.lib.GetThumb2(p)
	default:
		return nil, fmt.Errorf("invalid image type '%v'", imgType)
	}

	if err != nil {
		return nil, err
	}
	return data, nil
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}
