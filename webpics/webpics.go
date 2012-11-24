package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/piclib"
	"launchpad.net/goamz/aws"
)

var indexFiles = []string{"index.tmpl", "templates/browsepics.tmpl"}
var zoomFiles = []string{"templates/zoompic.tmpl"}

const (
	OrigImg   = "orig"
	Thumb1Img = "thumb1"
	Thumb2Img = "thumb2"
)

const (
	libName = "rwc-piclib"
	addr    = "0.0.0.0:7777"
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

	h := &handler{
		cache:     make(map[string][]byte),
		indexTmpl: indexTmpl,
		zoomTmpl:  zoomTmpl,
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
	cache     map[string][]byte
	indexTmpl *template.Template
	zoomTmpl  *template.Template
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
	sort.Sort(newFirst(h.photos))
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		list := make([]*thumbData, len(h.photos))
		for i, p := range h.photos {
			list[i] = &thumbData{
				Path:  p.Meta,
				Date:  p.Taken.Format("Jan 2, 2006"),
				Index: i,
			}
		}

		err := h.indexTmpl.Execute(w, list)
		if err != nil {
			log.Fatal(err)
		}
	} else if strings.HasPrefix(r.URL.Path, "/zoom") {
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
	} else if strings.HasPrefix(r.URL.Path, "/static") {
		log.Printf("serving static file '%v'", r.URL.Path)
		http.ServeFile(w, r, r.URL.Path[1:])
	} else if strings.HasPrefix(r.URL.Path, "/addphotos") {
		mr, err := req.MultipartReader()
		if err != nil {
			log.Println(err)
			return
		}

		resps := []interface{}{}
		for part, err := mr.NextPart(); err == nil; {
			if name := part.FormName(); name == "" {
				continue
			} else if part.FileName() == "" {
				continue
			}

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
					respMeta["error"] = err
				} else {
					h.photos = append(h.photos, p)
				}
			}

			resps = append(resps, resp)
			part, err = mr.NextPart()
		}

		sort.Sort(newFirst(h.photos))
		data, _ := json.Marshal(resps)
		w.Write(data)
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
