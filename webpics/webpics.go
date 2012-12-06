package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"bytes"
	"time"

	"github.com/rwcarlsen/gallery/backend/amz"
	"github.com/rwcarlsen/gallery/piclib"
	"launchpad.net/goamz/aws"
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

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	auth, err := aws.EnvAuth()
	if err != nil {
		log.Fatal(err)
	}
	db := amz.New(auth, aws.USEast)
	lib := piclib.New(libName, db)

	h := &handler{
		lib: lib,
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

	h.photos = make([]*piclib.Photo, 0, len(names))
	picCh := make(chan *piclib.Photo)
	for _, nm := range names {
		go func(name string) {
			p, err := h.lib.GetPhoto(name)
			if err != nil {
				log.Print(err)
			}
			picCh <- p
		}(nm)
	}

	for _ = range names {
		if p := <-picCh; p != nil {
			h.photos = append(h.photos, p)
		}
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
	if err := indexTmpl.Execute(w, nil); err != nil {
		log.Fatal(err)
	}
}

func (h *handler) serveDynamic(w http.ResponseWriter, r *http.Request) {
	items := strings.Split(r.URL.Path, "/")
	if len(items) != 3 {
		log.Printf("invalid dynamic content request path %v", r.URL.Path)
		return
	}

	kind := r.URL.Path[len("/dynamic"):]
	if strings.HasPrefix(kind, "/pg") {
		pgNum, err := strconv.Atoi(kind[len("/pg"):])
		if err != nil {
			log.Println("invalid gallery page view request")
			return
		}

		start := picsPerPage * (pgNum - 1)
		end := min(start+picsPerPage, len(h.photos))
		list := make([]*thumbData, end-start)
		for i, p := range h.photos[start:end] {
			list[i] = &thumbData{
				Path:  p.Meta,
				Date:  p.Taken.Format("Jan 2, 2006"),
				Index: i + start,
			}
		}

		if err = picsTmpl.Execute(w, list); err != nil {
			log.Fatal(err)
		}
	} else if strings.HasPrefix(kind, "/page-nav") {
		n := len(h.photos)/picsPerPage + 1
		pages := make([]int, n)
		for i := range pages {
			pages[i] = i + 1
		}

		if err := pagenavTmpl.Execute(w, pages); err != nil {
			log.Fatal(err)
		}
	} else if strings.HasPrefix(kind, "/num-pages") {
		n := len(h.photos)/picsPerPage + 1
		fmt.Fprint(w, n)
	} else if strings.HasPrefix(kind, "/num-pics") {
		fmt.Fprint(w, len(h.photos))
	} else if strings.HasPrefix(kind, "/time-nav") {
		years := make([]*year, 0)
		maxYear := h.photos[0].Taken.Year()
		minYear := h.photos[len(h.photos)-1].Taken.Year()
		lastMinMonth := h.photos[len(h.photos)-1].Taken.Month()

		var last, pg int
		for y := maxYear; y > minYear; y-- {
			yr := &year{Year: y}
			for m := time.December; m >= time.January; m-- {
				pg, last = h.pageOf(last, time.Date(y, m, 1, 0, 0, 0, 0, time.Local))
				yr.Months = append(yr.Months, &month{Page: pg, Name: m.String()[:3]})
			}
			yr.reverseMonths()
			yr.StartPage = yr.Months[0].Page
			years = append(years, yr)
		}

		yr := &year{Year: minYear}
		for m := time.December; m >= lastMinMonth; m-- {
			pg, last = h.pageOf(last, time.Date(minYear, m, 1, 0, 0, 0, 0, time.Local))
			yr.Months = append(yr.Months, &month{Page: pg, Name: m.String()[:3]})
		}
		yr.reverseMonths()
		yr.StartPage = yr.Months[0].Page
		years = append(years, yr)

		if err := timenavTmpl.Execute(w, years); err != nil {
			log.Fatal(err)
		}
	}
}

func (h *handler) pageOf(start int, t time.Time) (page, last int) {
	for i, p := range h.photos[start:] {
		pg := (i+start)/picsPerPage + 1

		if p.Taken.Before(t) {
			return pg, i + start
		}
	}
	return len(h.photos)/picsPerPage + 1, len(h.photos)
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

	if err := zoomTmpl.Execute(w, pData); err != nil {
		log.Fatal(err)
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
