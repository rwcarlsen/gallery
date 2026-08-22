package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/rwcarlsen/gallery/piclib"
)

//go:embed data
var dataFS embed.FS

const picsPerPage = 24

var (
	zoomTmpl  *template.Template
	gridTmpl  *template.Template
	utilTmpl  *template.Template
	loginTmpl *template.Template
	slidepage []byte
)

var (
	basePhotos []*piclib.Pic
	noedit     bool
	addr       string
	all        bool
)

type Photo struct {
	*piclib.Pic
	Index   int
	Notes   string
	CanEdit bool
	User    string
}

func (p Photo) Date() string {
	return p.Taken.Format("Jan 2, 2006")
}

func parseTemplates() {
	read := func(name string) string {
		b, err := dataFS.ReadFile(name)
		check(err)
		return string(b)
	}
	util := read("data/util.html")
	utilTmpl = template.Must(template.New("util").Parse(util))

	gridTmpl = template.Must(template.New("index").Parse(read("data/index.html")))
	template.Must(gridTmpl.Parse(util))

	zoomTmpl = template.Must(template.New("zoompic").Parse(read("data/zoompic.html")))
	template.Must(zoomTmpl.Parse(util))

	loginTmpl = template.Must(template.New("login").Parse(read("data/login.html")))
	template.Must(loginTmpl.Parse(util))
}

func runserve(l net.Listener, args []string) {
	parseTemplates()
	var err error
	slidepage, err = dataFS.ReadFile("data/slideshow.html")
	check(err)

	initSessions()
	loadPics(args)

	static, err := fs.Sub(dataFS, "data/static")
	check(err)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("GET /login", loginGet)
	mux.HandleFunc("POST /login", loginPost)
	mux.HandleFunc("POST /logout", logoutPost)
	mux.HandleFunc("GET /{$}", auth(HomeHandler))
	mux.HandleFunc("GET /photo/{kind}/{id}", auth(PhotoHandler))
	mux.HandleFunc("GET /dynamic/pg", auth(CurrPageHandler))
	mux.HandleFunc("GET /dynamic/pg/{pg}", auth(PageHandler))
	mux.HandleFunc("GET /dynamic/zoom/{index}", auth(ZoomHandler))
	mux.HandleFunc("GET /dynamic/page-nav", auth(PageNavHandler))
	mux.HandleFunc("GET /dynamic/time-nav", auth(TimeNavHandler))
	mux.HandleFunc("GET /dynamic/set-page/{page}", auth(SetPageHandler))
	mux.HandleFunc("GET /dynamic/stat/{stat}", auth(StatHandler))
	mux.HandleFunc("POST /dynamic/save-notes/{picIndex}", auth(NotesHandler))
	mux.HandleFunc("GET /dynamic/slideshow", auth(SlideshowHandler))
	mux.HandleFunc("GET /dynamic/next-slide", auth(NextSlideHandler))
	mux.HandleFunc("GET /dynamic/slide-style", auth(SlideStyleHandler))
	mux.HandleFunc("GET /dynamic/clickpic/{id}", auth(clickpicHandler))

	server := &http.Server{Addr: addr, Handler: mux}
	err = server.Serve(l)
	check(err)
}

func loadPics(args []string) {
	var pics []*piclib.Pic
	var err error
	if all {
		pics, err = lib.List(0, 0)
		check(err)
	} else {
		pics = idsOrStdin(args)
	}
	for _, p := range pics {
		switch p.Ext() {
		case ".avi", ".m4v":
			continue
		}
		basePhotos = append(basePhotos, p)
	}
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if err := gridTmpl.Execute(w, map[string]string{"User": c.userName()}); err != nil {
		log.Print(err)
	}
}

func clickpicHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		return
	}
	if _, ok := c.picMap[id]; !ok {
		return
	}
	fmt.Println(id)
}

func PhotoHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, ok := c.picMap[id]; !ok {
		http.NotFound(w, r)
		return
	}
	switch r.PathValue("kind") {
	case "orig":
		err = writeImg(w, id, false)
	case "thumb":
		err = writeImg(w, id, true)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		log.Print(err)
	}
}

func writeImg(w io.Writer, id int, thumb bool) error {
	p, err := lib.Open(id)
	if err != nil {
		return err
	}
	if thumb {
		data, err := p.Thumb()
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	rc, err := p.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(w, rc)
	return err
}

func CurrPageHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	fmt.Fprint(w, c.CurrPage)
}

func PageHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if err := c.servePage(w, r.PathValue("pg")); err != nil {
		log.Print(err)
	}
}

func NotesHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if !c.canEdit() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := c.saveNotes(r, r.PathValue("picIndex")); err != nil {
		log.Print(err)
	}
}

func NextSlideHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if err := c.serveSlide(w); err != nil {
		log.Print(err)
	}
}

func SlideStyleHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(c.slideStyle()))
}

func SlideshowHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(slidepage)
}

func ZoomHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if err := c.serveZoom(w, r.PathValue("index")); err != nil {
		http.NotFound(w, r)
	}
}

func PageNavHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if err := c.servePageNav(w); err != nil {
		log.Print(err)
	}
}

func StatHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	c.serveStat(w, r.PathValue("stat"))
}

func SetPageHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	c.CurrPage = r.PathValue("page")
}

func TimeNavHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if err := c.serveTimeNav(w); err != nil {
		log.Print(err)
	}
}
