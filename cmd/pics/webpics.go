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
	"regexp"
	"strconv"
	"strings"

	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"

	"github.com/rwcarlsen/gallery/internal/infer"
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
	Match   string
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
	mux.HandleFunc("GET /dynamic/search", auth(SearchHandler))
	mux.HandleFunc("GET /dynamic/meta/{id}", auth(MetaHandler))
	mux.HandleFunc("GET /dynamic/faces/{picId}", auth(FacesHandler))
	mux.HandleFunc("GET /dynamic/person/{name}", auth(PersonHandler))
	mux.HandleFunc("GET /dynamic/face/{faceId}/similar", auth(FaceSimilarHandler))
	mux.HandleFunc("POST /dynamic/face/{faceId}/name", auth(FaceNameHandler))
	mux.HandleFunc("GET /photo/face/{faceId}", auth(FacePhotoHandler))
	mux.HandleFunc("GET /dynamic/caps", auth(CapsHandler))

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
	data := map[string]string{"User": c.userName(), "Filter": c.Filter}
	if err := gridTmpl.Execute(w, data); err != nil {
		log.Print(err)
	}
}

func canSearch() bool {
	n, err := lib.ClipCount()
	return err == nil && n > 0 && infer.HasCLIP(lib.ModelsDir())
}

var searchEng *infer.Engine

func getSearchEng() (*infer.Engine, error) {
	if searchEng != nil {
		return searchEng, nil
	}
	e, err := infer.Open(lib.ModelsDir(), true, false)
	if err != nil {
		return nil, err
	}
	searchEng = e
	return e, nil
}

func CapsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"search": canSearch()})
}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		c.resetFilter()
		fmt.Fprint(w, "ok")
		return
	}
	ids, why, err := searchIDs(c, q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.filterIDs(ids)
	c.Filter = q
	c.why = why
	fmt.Fprint(w, "ok")
}

func searchIDs(c *gallery, q string) ([]int, map[int]hitWhy, error) {
	var ids []int
	why := map[int]hitWhy{}
	add := func(id int, fn func(*hitWhy)) {
		if _, ok := why[id]; !ok {
			ids = append(ids, id)
		}
		h := why[id]
		fn(&h)
		why[id] = h
	}

	people, err := lib.ListPeople()
	if err != nil {
		return nil, nil, err
	}
	for _, p := range people {
		if !strings.EqualFold(p.Name, q) {
			continue
		}
		pids, err := lib.PicsByPerson(p.Name)
		if err != nil {
			return nil, nil, err
		}
		for _, id := range pids {
			add(id, func(h *hitWhy) { h.Person = true })
		}
		break
	}

	pics := make([]*piclib.Pic, len(c.visible))
	for i, p := range c.visible {
		pics[i] = p.Pic
	}
	matched, err := lib.FilterPics(pics, "(?i)"+regexp.QuoteMeta(q), "")
	if err != nil {
		return nil, nil, err
	}
	for _, p := range matched {
		add(p.Id, func(h *hitWhy) { h.Notes = true })
	}

	if canSearch() {
		eng, err := getSearchEng()
		if err == nil {
			if vec, err := eng.EmbedText(q); err == nil {
				if hits, err := lib.SimilarClip(vec, piclib.DefaultClipMin); err == nil {
					for _, hit := range hits {
						add(hit.ID, func(h *hitWhy) { h.Clip = hit.Score })
					}
				}
			}
		}
	}
	return ids, why, nil
}

func MetaHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || !c.visiblePic(id) {
		http.NotFound(w, r)
		return
	}
	p, err := lib.Open(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tags, err := p.FileMeta()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
}

func FacesHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	id, err := strconv.Atoi(r.PathValue("picId"))
	if err != nil || !c.visiblePic(id) {
		http.NotFound(w, r)
		return
	}
	faces, err := lib.FacesByPic(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	type chip struct {
		ID   int     `json:"id"`
		Name string  `json:"name"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		W    float64 `json:"w"`
		H    float64 `json:"h"`
	}
	out := make([]chip, 0, len(faces))
	for _, f := range faces {
		out = append(out, chip{f.FaceID, f.Name, f.X, f.Y, f.W, f.H})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func PersonHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	ids, err := lib.PicsByPerson(r.PathValue("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.filterIDs(ids)
	c.Filter = r.PathValue("name")
	c.why = make(map[int]hitWhy, len(ids))
	for _, id := range ids {
		c.why[id] = hitWhy{Person: true}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func FaceSimilarHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	fid, err := strconv.Atoi(r.PathValue("faceId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := lib.GetFace(fid)
	if err != nil || f == nil || !c.visiblePic(f.PicID) {
		http.NotFound(w, r)
		return
	}
	if f.Name != "" {
		ids, err := lib.PicsByPerson(f.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		c.filterIDs(ids)
		c.Filter = f.Name
		c.why = make(map[int]hitWhy, len(ids))
		for _, id := range ids {
			c.why[id] = hitWhy{Person: true}
		}
		fmt.Fprint(w, "ok")
		return
	}
	hits, err := lib.SimilarFaces(f.Vec, piclib.DefaultFaceMin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ids := make([]int, 0, len(hits))
	seen := map[int]bool{}
	for _, h := range hits {
		if !seen[h.PicID] {
			seen[h.PicID] = true
			ids = append(ids, h.PicID)
		}
	}
	c.filterIDs(ids)
	c.Filter = "similar face"
	c.why = make(map[int]hitWhy, len(ids))
	for _, id := range ids {
		c.why[id] = hitWhy{Face: true}
	}
	fmt.Fprint(w, "ok")
}

func FaceNameHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	if !c.canEdit() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	fid, err := strconv.Atoi(r.PathValue("faceId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := lib.GetFace(fid)
	if err != nil || f == nil || !c.visiblePic(f.PicID) {
		http.NotFound(w, r)
		return
	}
	name, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := lib.AssignPerson(fid, string(bytes.TrimSpace(name))); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	n, err := lib.PropagatePerson(string(bytes.TrimSpace(name)), piclib.DefaultFaceMin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, n)
}

func FacePhotoHandler(w http.ResponseWriter, r *http.Request) {
	c := mustGallery(w, r)
	fid, err := strconv.Atoi(r.PathValue("faceId"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	f, err := lib.GetFace(fid)
	if err != nil || f == nil || !c.visiblePic(f.PicID) {
		http.NotFound(w, r)
		return
	}
	p, err := lib.Open(f.PicID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	thumb, err := p.Thumb()
	if err != nil || len(thumb) == 0 {
		http.NotFound(w, r)
		return
	}
	img, _, err := image.Decode(bytes.NewReader(thumb))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jpeg, err := piclib.CropNormJPEG(img, f.X, f.Y, f.W, f.H)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Write(jpeg)
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
