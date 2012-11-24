
package main

import (
	"fmt"
	"log"
	"strings"
	"html/template"
	"net/http"
	pth "path"

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
	libName = "rwc-webpics"
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

	http.Handle("/", h)
	err = http.ListenAndServe("0.0.0.0:7777", nil)
	if err != nil {
		log.Fatal(err)
	}
}

type handler struct {
	cache map[string][]byte
	indexTmpl *template.Template
	lib *piclib.Library
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		photoList, err := h.lib.ListPhotosN(20)
		if err != nil {
			log.Fatal(err)
		}

		list := make([]string, len(photoList))
		for i, p := range photoList {
			list[i] = pth.Join("piclib/thumb1", p.Meta)
		}
		if err != nil {
			log.Fatal(err)
		}

		err = h.indexTmpl.Execute(w, list)
		if err != nil {
			log.Fatal(err)
		}
	} else if strings.HasPrefix(r.URL.Path, "/static") {
		log.Printf("serving static file '%v'", r.URL.Path)
		http.ServeFile(w, r, r.URL.Path[1:])
	} else if strings.HasPrefix(r.URL.Path, "/piclib") {
		if _, ok := h.cache[r.URL.Path]; !ok {
			if err := h.fetchImg(r.URL.Path); err != nil {
				log.Fatal(err)
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
