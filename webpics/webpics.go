package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/piclib"
	"launchpad.net/goamz/aws"
)

var indexFiles = []string{"index.tmpl", "templates/pagination.tmpl"}
var picFiles = []string{"templates/browsepics.tmpl"}
var zoomFiles = []string{"templates/zoompic.tmpl"}

const (
	MetaFile  = "meta"
	OrigImg   = "orig"
	Thumb1Img = "thumb1"
	Thumb2Img = "thumb2"
)

const (
	libName = "rwc-piclib2"
	addr    = "0.0.0.0:7777"
)

const (
	picsPerPage = 24
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	lib := piclib.New(libName, db)

	indexTmpl := template.Must(template.ParseFiles(indexFiles...))
	zoomTmpl := template.Must(template.ParseFiles(zoomFiles...))
	picsTmpl := template.Must(template.ParseFiles(picFiles...))

	h := &handler{
		indexTmpl: indexTmpl,
		zoomTmpl:  zoomTmpl,
		picsTmpl:  picsTmpl,
		lib:       lib,
	}
	h.updateLib()

	http.Handle("/", h)
	log.Printf("listening on %v", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

type newFirst []*piclib.Photo

func (pl newFirst) Less(i, j int) bool {
	return pl[i].Taken.After(pl[j].Taken)
}

func (pl newFirst) Len() int {
	return len(pl)
}

func (pl newFirst) Swap(i, j int) {
	pl[i], pl[j] = pl[j], pl[i]
}

type handler struct {
	indexTmpl *template.Template
	zoomTmpl  *template.Template
	picsTmpl  *template.Template
	lib       *piclib.Library
	photos    []*piclib.Photo
}

type thumbData struct {
	Path  string
	Date  string
	Index int
}

func (h *handler) updateLib() {
	var err error
	if h.photos, err = h.lib.ListPhotosN(50000); err != nil {
		log.Println(err)
	}
	if len(h.photos) > 0 {
		sort.Sort(newFirst(h.photos))
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		h.serveHome(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/pg") {
		h.servePage(w, r)
	} else if strings.HasPrefix(r.URL.Path, "/zoom") {
		h.serveZoom(w, r)
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
	pages := make([]int, len(h.photos) / picsPerPage + 1)
	for i := range pages {
		pages[i] = i + 1
	}

	if err := h.indexTmpl.Execute(w, pages); err != nil {
		log.Fatal(err)
	}
}

func (h *handler) servePage(w http.ResponseWriter, r *http.Request) {
	items := strings.Split(r.URL.Path, "pg")
	if len(items) != 2 {
		log.Println("invalid gallery page view request")
		return
	}
	pgNum, err := strconv.Atoi(items[1])
	if err != nil {
		log.Println("invalid gallery page view request")
		return
	}

	start := picsPerPage * (pgNum - 1)
	end := min(start + picsPerPage, len(h.photos))
	list := make([]*thumbData, end - start)
	for i, p := range h.photos[start:end] {
		list[i] = &thumbData{
			Path:  p.Meta,
			Date:  p.Taken.Format("Jan 2, 2006"),
			Index: i,
		}
	}

	if err = h.picsTmpl.Execute(w, list); err != nil {
		log.Fatal(err)
	}
}

func (h *handler) serveZoom(w http.ResponseWriter, r *http.Request) {
	items := strings.Split(r.URL.Path[1:], "/")
	if len(items) != 2 {
		log.Printf("Invalid zoom request path '%v'", r.URL.Path)
	}

	i, _ := strconv.Atoi(items[1])
	p := h.photos[i]
	pData := &thumbData{
		Path:  p.Meta,
		Date:  p.Taken.Format("Jan 2, 2006"),
		Index: i,
	}

	if err := h.zoomTmpl.Execute(w, pData); err != nil {
		log.Fatal(err)
	}
}

func (h *handler) serveAddPhotos(w http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		log.Println(err)
		return
	}

	resps := []interface{}{}
	part, err := mr.NextPart()
	for {
		if part.FormName() == "" {
			continue
		} else if part.FileName() == "" {
			continue
		}
		name := part.FileName()

		data, err := ioutil.ReadAll(part)
		respMeta := map[string]interface{}{
			"name": name,
			"size": len(data),
		}

		if err != nil {
			log.Println(err)
			respMeta["error"] = err
		} else {
			p, err := h.lib.AddPhoto(name, data)
			if err != nil {
				respMeta["error"] = err.Error()
			} else {
				h.photos = append(h.photos, p)
			}
		}

		resps = append(resps, respMeta)
		if part, err = mr.NextPart(); err != nil {
			break
		}
	}

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
