package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/rwcarlsen/gallery/piclib"
)

type context struct {
	photos    []Photo
	CurrPage  string
	random    []int
	randIndex int
}

func newContext(pics []Photo) *context {
	c := &context{
		photos:   pics,
		CurrPage: "1",
	}
	return c
}

func (c *context) saveNotes(r *http.Request, picIndex string) error {
	i, err := strconv.Atoi(picIndex)
	if err != nil {
		return fmt.Errorf("invalid save notes request: %v", r.URL)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	r.Body.Close()

	p := c.photos[i]
	err = piclib.WriteNotes(p.Path, string(data))
	if err != nil {
		return err
	}
	return nil
}

func (c *context) initRand() {
	if c.random == nil || len(c.random) > len(c.photos) {
		c.random = rand.Perm(len(c.photos))
		c.randIndex = 0
	}
}

func (c *context) serveSlide(w http.ResponseWriter) error {
	c.initRand()
	p := c.photos[c.random[c.randIndex]]
	if c.randIndex++; c.randIndex == len(c.photos) {
		c.randIndex = 0
	}
	return writeImg(w, p.Name, false)
}

func (c *context) servePage(w http.ResponseWriter, pg string) error {
	pgNum, err := strconv.Atoi(pg)
	if err != nil {
		return fmt.Errorf("invalid gallery page view request: %v", pg)
	}

	start := picsPerPage * (pgNum - 1)
	end := min(start+picsPerPage, len(c.photos))
	list := make([]Photo, end-start)
	for i, p := range c.photos[start:end] {
		p.Index = i + start
		list[i] = p
	}

	if err = utilTmpl.ExecuteTemplate(w, "picgrid", list); err != nil {
		return err
	}
	c.CurrPage = pg
	return nil
}

func (c *context) serveZoom(w http.ResponseWriter, index string) error {
	i, _ := strconv.Atoi(index)
	p := c.photos[i]
	p.Index = i
	return zoomTmpl.Execute(w, p)
}

func (c *context) servePageNav(w http.ResponseWriter) error {
	n := len(c.photos)/picsPerPage + 1
	pages := make([]int, n)
	for i := range pages {
		pages[i] = i + 1
	}

	return utilTmpl.ExecuteTemplate(w, "pagenav", pages)
}

func (c *context) serveStat(w http.ResponseWriter, stat string) {
	switch stat {
	case "num-pages":
		n := len(c.photos)/picsPerPage + 1
		fmt.Fprint(w, n)
	case "pics-per-page":
		fmt.Fprint(w, picsPerPage)
	case "num-pics":
		fmt.Fprint(w, len(c.photos))
	default:
		fmt.Fprintf(w, "invalid stat '%v'", stat)
	}
}

func (c *context) serveTimeNav(w http.ResponseWriter) error {
	if len(c.photos) == 0 {
		return nil
	}
	years := make([]*year, 0)
	maxYear := c.photos[0].Taken.Year()
	minYear := c.photos[len(c.photos)-1].Taken.Year()
	lastMinMonth := c.photos[len(c.photos)-1].Taken.Month()
	if maxYear-minYear > 20 {
		minYear = maxYear - 20
		lastMinMonth = 12
	}

	var last, pg int
	for y := maxYear; y > minYear; y-- {
		yr := &year{Year: y}
		for m := time.December; m >= time.January; m-- {
			pg, last = c.pageOf(last, time.Date(y, m, 1, 0, 0, 0, 0, time.Local))
			yr.Months = append(yr.Months, &month{Page: pg, Name: m.String()[:3]})
		}
		yr.reverseMonths()
		yr.StartPage = yr.Months[0].Page
		years = append(years, yr)
	}

	yr := &year{Year: minYear}
	for m := time.December; m >= lastMinMonth; m-- {
		pg, last = c.pageOf(last, time.Date(minYear, m, 1, 0, 0, 0, 0, time.Local))
		yr.Months = append(yr.Months, &month{Page: pg, Name: m.String()[:3]})
	}
	yr.reverseMonths()
	yr.StartPage = yr.Months[0].Page
	years = append(years, yr)

	return utilTmpl.ExecuteTemplate(w, "timenav", years)
}

func (c *context) pageOf(start int, t time.Time) (page, last int) {
	if len(c.photos) == 0 {
		return
	}
	for i, p := range c.photos[start:] {
		pg := (i+start)/picsPerPage + 1

		if p.Taken.Before(t) {
			return pg, i + start
		}
	}
	return len(c.photos)/picsPerPage + 1, len(c.photos)
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

type month struct {
	Name string
	Page int
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
