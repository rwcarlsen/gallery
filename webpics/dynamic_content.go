
package main

import (
	"log"
	"fmt"
	"time"
	"strconv"
	"net/http"
	"html/template"

	"github.com/rwcarlsen/gallery/piclib"
)

var (
	zoomTmpl = template.Must(template.ParseFiles("templates/zoompic.tmpl"))
	picsTmpl = template.Must(template.ParseFiles("templates/browsepics.tmpl"))
	pagenavTmpl = template.Must(template.ParseFiles("templates/pagination.tmpl"))
	timenavTmpl = template.Must(template.ParseFiles("templates/timenav.tmpl"))
)

type context struct {
	photos []*piclib.Photo
	HideDateless bool
	CurrPage string
}

func (c *context) toggleDateless() {
	c.HideDateless = !c.HideDateless
	if c.HideDateless {
		newlist := make([]*piclib.Photo, 0, len(c.photos))
		for _, p := range c.photos {
			if p.LegitTaken() {
				newlist = append(newlist, p)
			}
		}
		c.photos = newlist
	} else {
		c.resetPics()
	}
}

func (c *context) resetPics() {
	c.photos = allPhotos
}

func (c *context) servePage(w http.ResponseWriter, pg string) {
	pgNum, err := strconv.Atoi(pg)
	if err != nil {
		log.Println("invalid gallery page view request")
		return
	}

	start := picsPerPage * (pgNum - 1)
	end := min(start+picsPerPage, len(c.photos))
	list := make([]*thumbData, end-start)
	for i, p := range c.photos[start:end] {
		list[i] = &thumbData{
			Path:  p.Meta,
			Date:  p.Taken.Format("Jan 2, 2006"),
			Index: i + start,
		}
	}

	if err = picsTmpl.Execute(w, list); err != nil {
		log.Fatal(err)
	}
	c.CurrPage = pg
}

func (c *context) serveZoom(w http.ResponseWriter, index string) {
	i , _ := strconv.Atoi(index)
	p := c.photos[i]
	pData := &thumbData{
		Path:  p.Meta,
		Date:  p.Taken.Format("Jan 2, 2006"),
		Index: i,
	}

	if err := zoomTmpl.Execute(w, pData); err != nil {
		log.Fatal(err)
	}
}

func (c *context) servePageNav(w http.ResponseWriter) {
	n := len(c.photos)/picsPerPage + 1
	pages := make([]int, n)
	for i := range pages {
		pages[i] = i + 1
	}

	if err := pagenavTmpl.Execute(w, pages); err != nil {
		log.Fatal(err)
	}
}

func (c *context) serveStat(w http.ResponseWriter, stat string) {
	switch stat {
	case "num-pages":
		n := len(c.photos)/picsPerPage + 1
		fmt.Fprint(w, n)
	case "num-pics":
		fmt.Fprint(w, len(c.photos))
	case "hiding-dateless":
		fmt.Fprint(w, c.HideDateless)
	default:
		fmt.Fprintf(w, "invalid stat '%v'", stat)
	}
}

func (c *context) serveTimeNav(w http.ResponseWriter) {
	years := make([]*year, 0)
	maxYear := c.photos[0].Taken.Year()
	minYear := c.photos[len(c.photos)-1].Taken.Year()
	lastMinMonth := c.photos[len(c.photos)-1].Taken.Month()

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

	if err := timenavTmpl.Execute(w, years); err != nil {
		log.Fatal(err)
	}
}

func (c *context) pageOf(start int, t time.Time) (page, last int) {
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
