
package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"html/template"
	"net/http"

	"launchpad.net/goamz/aws"
	"github.com/rwcarlsen/gallery/piclib"
	"github.com/rwcarlsen/gallery/backend/amz"
)

var indexFiles = []string{"index.tmpl", "templates/browsepics.tmpl"}

const (
	OrigImg = "orig"
	Thumb1Img = "thumb1"
	Thumb2Img = "thumb2"
)

const (
	libName = "rwc-piclib"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// setup piclib
	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	lib := piclib.New(libName, db)

	tmpl := template.Must(template.ParseFiles(indexFiles...))

	h := &handler{
		cache: make(map[string][]byte),
		indexTmpl: tmpl,
		lib: lib,
	}
	h.updateLib()

	http.Handle("/", h)
	err = http.ListenAndServe("0.0.0.0:7777", nil)
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
	cache map[string][]byte
	indexTmpl *template.Template
	lib *piclib.Library
	photos []*piclib.Photo
}

type thumbData struct {
	Path string
	Date string
}

func (h *handler) updateLib() {
	var err error
	if h.photos, err = h.lib.ListPhotosN(50000); err != nil {
		log.Println(err)
	}
	sort.Sort(newFirst(h.photos))
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		list := make([]*thumbData, len(h.photos))
		for i, p := range h.photos {
			list[i] = &thumbData{p.Meta, p.Taken.Format("Jan 2, 2006")}
		}

		err := h.indexTmpl.Execute(w, list)
		if err != nil {
			log.Fatal(err)
		}
	} else if strings.HasPrefix(r.URL.Path, "/static") {
		log.Printf("serving static file '%v'", r.URL.Path)
		http.ServeFile(w, r, r.URL.Path[1:])
	} else if strings.HasPrefix(r.URL.Path, "/piclib") {
		if _, ok := h.cache[r.URL.Path]; !ok {
			if err := h.fetchImg(r.URL.Path); err != nil {
				log.Print(err)
			}
		}
		w.Write(h.cache[r.URL.Path])
	} else {
		log.Printf("Invalid request path '%v'", r.URL.Path)
	}
}

func (h *handler) fetchImg(path string) error {
	items := strings.Split(path[1:], "/")
	if len(items) != 3 {
		return fmt.Errorf("invalid piclib resource '%v'", path)
	}
	imgType, pName := items[1], items[2]

	p, err := h.lib.GetPhoto(pName)
	if err != nil {
		log.Println("pName: ", pName)
		return err
	}

	var data []byte
	switch imgType {
	case OrigImg:
		data, err = h.lib.GetOriginal(p)
	case Thumb1Img:
		data, err = h.lib.GetThumb1(p)
	case Thumb2Img:
		data, err = h.lib.GetThumb2(p)
	default:
		return fmt.Errorf("invalid image type '%v'", imgType)
	}

	if err != nil {
		return err
	}
	h.cache[path] = data
	return nil
}
