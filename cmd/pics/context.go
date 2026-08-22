package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/rwcarlsen/gallery/piclib"
)

var rots = map[int]int{
	1: 0,
	2: 0,
	3: 180,
	4: 180,
	5: 90,
	6: 90,
	7: 270,
	8: 270,
}

type gallery struct {
	user      *piclib.User
	visible   []*Photo
	photos    []*Photo
	picMap    map[int]*Photo
	CurrPage  string
	random    []int
	randIndex int
}

func newGallery(u *piclib.User) *gallery {
	pics := basePhotos
	var err error
	if u != nil {
		pics, err = lib.VisiblePics(basePhotos, u)
		check(err)
	}
	notes, err := lib.NotesMap()
	check(err)
	edit := !noedit && (u == nil || u.CanWrite)
	name := ""
	if u != nil {
		name = u.Name
	}
	c := &gallery{
		user:     u,
		photos:   make([]*Photo, 0, len(pics)),
		picMap:   map[int]*Photo{},
		CurrPage: "1",
	}
	for _, p := range pics {
		ph := &Photo{Pic: p, Notes: notes[p.Id], CanEdit: edit, User: name}
		c.photos = append(c.photos, ph)
		c.picMap[p.Id] = ph
	}
	c.visible = append([]*Photo{}, c.photos...)
	return c
}

func (c *gallery) rebuild() {
	c.picMap = map[int]*Photo{}
	for _, p := range c.photos {
		c.picMap[p.Id] = p
	}
	c.CurrPage = "1"
	c.random = nil
}

func (c *gallery) resetFilter() {
	c.photos = append([]*Photo{}, c.visible...)
	c.rebuild()
}

func (c *gallery) filterIDs(ids []int) {
	byID := make(map[int]*Photo, len(c.visible))
	for _, p := range c.visible {
		byID[p.Id] = p
	}
	c.photos = c.photos[:0]
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			c.photos = append(c.photos, p)
		}
	}
	c.rebuild()
}

func (c *gallery) visiblePic(id int) bool {
	for _, p := range c.visible {
		if p.Id == id {
			return true
		}
	}
	return false
}

func (c *gallery) userName() string {
	if c.user == nil {
		return ""
	}
	return c.user.Name
}

func (c *gallery) canEdit() bool {
	if noedit {
		return false
	}
	return c.user == nil || c.user.CanWrite
}

func (c *gallery) numPages() int {
	if len(c.photos) == 0 {
		return 1
	}
	return (len(c.photos) + picsPerPage - 1) / picsPerPage
}

func (c *gallery) saveNotes(r *http.Request, picIndex string) error {
	i, err := strconv.Atoi(picIndex)
	if err != nil || i < 0 || i >= len(c.photos) {
		return fmt.Errorf("invalid save notes request: %v", r.URL)
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body.Close()
	p := c.photos[i]
	if err := p.SetNotes(string(data)); err != nil {
		return err
	}
	p.Notes = string(data)
	return nil
}

func (c *gallery) initRand() {
	if c.random == nil || len(c.random) != len(c.photos) {
		c.random = rand.Perm(len(c.photos))
		c.randIndex = 0
	}
}

func (c *gallery) serveSlide(w http.ResponseWriter) error {
	if len(c.photos) == 0 {
		return fmt.Errorf("no photos")
	}
	c.initRand()
	p := c.photos[c.random[c.randIndex]]
	if c.randIndex++; c.randIndex == len(c.photos) {
		c.randIndex = 0
	}
	return writeImg(w, p.Id, false)
}

func (c *gallery) slideStyle() string {
	if len(c.photos) == 0 {
		return ""
	}
	c.initRand()
	p := c.photos[c.random[c.randIndex]]
	deg := rots[p.Orient]
	return fmt.Sprintf("transform:rotate(%ddeg)", deg)
}

func (c *gallery) servePage(w http.ResponseWriter, pg string) error {
	pgNum, err := strconv.Atoi(pg)
	if err != nil {
		return fmt.Errorf("invalid gallery page view request: %v", pg)
	}
	if pgNum < 1 {
		pgNum = 1
	}
	start := picsPerPage * (pgNum - 1)
	if start > len(c.photos) {
		start = len(c.photos)
	}
	end := start + picsPerPage
	if end > len(c.photos) {
		end = len(c.photos)
	}
	list := make([]Photo, end-start)
	for i, p := range c.photos[start:end] {
		pp := *p
		pp.Index = i + start
		list[i] = pp
	}
	if err = utilTmpl.ExecuteTemplate(w, "picgrid", list); err != nil {
		return err
	}
	c.CurrPage = strconv.Itoa(pgNum)
	return nil
}

func (c *gallery) serveZoom(w http.ResponseWriter, index string) error {
	i, err := strconv.Atoi(index)
	if err != nil || i < 0 || i >= len(c.photos) {
		return fmt.Errorf("invalid zoom index %v", index)
	}
	p := *c.photos[i]
	p.Index = i
	p.CanEdit = c.canEdit()
	p.User = c.userName()
	return zoomTmpl.Execute(w, p)
}

func (c *gallery) pageLabel(pg int) string {
	if pg < 1 {
		pg = 1
	}
	start := picsPerPage * (pg - 1)
	if start >= len(c.photos) {
		return "—"
	}
	t := c.photos[start].Taken
	if t.IsZero() || t.Unix() == 0 {
		return "Unknown date"
	}
	return t.Format("2006 · Jan")
}

func (c *gallery) servePageNav(w http.ResponseWriter) error {
	pg, _ := strconv.Atoi(c.CurrPage)
	return utilTmpl.ExecuteTemplate(w, "pagenav", struct{ Label string }{c.pageLabel(pg)})
}

func (c *gallery) serveStat(w http.ResponseWriter, stat string) {
	switch stat {
	case "num-pages":
		fmt.Fprint(w, c.numPages())
	case "pics-per-page":
		fmt.Fprint(w, picsPerPage)
	case "num-pics":
		fmt.Fprint(w, len(c.photos))
	default:
		fmt.Fprintf(w, "invalid stat '%v'", stat)
	}
}

func (c *gallery) serveTimeNav(w http.ResponseWriter) error {
	type month struct {
		Name string
		Page int
	}
	type year struct {
		Year      int
		StartPage int
		Months    []*month
	}
	var years []*year
	var cur *year
	var curMonth time.Month
	for i, p := range c.photos {
		if p.Taken.IsZero() || p.Taken.Unix() == 0 {
			continue
		}
		y, m := p.Taken.Year(), p.Taken.Month()
		pg := i/picsPerPage + 1
		if cur == nil || cur.Year != y {
			cur = &year{Year: y, StartPage: pg}
			years = append(years, cur)
			curMonth = 0
		}
		if curMonth != m {
			cur.Months = append(cur.Months, &month{Name: m.String()[:3], Page: pg})
			curMonth = m
		}
	}
	for _, y := range years {
		for i, j := 0, len(y.Months)-1; i < j; i, j = i+1, j-1 {
			y.Months[i], y.Months[j] = y.Months[j], y.Months[i]
		}
	}
	return utilTmpl.ExecuteTemplate(w, "timenav", years)
}
