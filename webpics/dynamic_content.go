
package main

import (
	"log"
	"fmt"
	"time"
	"strconv"
	"strings"
	"net/http"
	"encoding/json"

	"github.com/rwcarlsen/gallery/piclib"
)

type context struct {
	h *handler
	photos []*piclib.Photo
	HideDateless bool
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
	c.photos = c.h.photos
}

func (c *context) servePage(w http.ResponseWriter, r *http.Request) {
	pgNum, err := strconv.Atoi(r.URL.Path[len("/dynamic/pg"):])
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
}

func (c *context) servePageNav(w http.ResponseWriter, r *http.Request) {
	n := len(c.photos)/picsPerPage + 1
	pages := make([]int, n)
	for i := range pages {
		pages[i] = i + 1
	}

	if err := pagenavTmpl.Execute(w, pages); err != nil {
		log.Fatal(err)
	}
}

func (c *context) serveNumPages(w http.ResponseWriter, r *http.Request) {
	n := len(c.photos)/picsPerPage + 1
	fmt.Fprint(w, n)
}

func (c *context) serveNumPics(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, len(c.photos))
}

func (c *context) serveTimeNav(w http.ResponseWriter, r *http.Request) {
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


func (c *context) servePhoto(w http.ResponseWriter, r *http.Request) {
	data, err := c.fetchImg(r.URL.Path)
	if err != nil {
		log.Print(err)
		return
	}
	w.Write(data)
}

func (c *context) fetchImg(path string) ([]byte, error) {
	items := strings.Split(path[1:], "/")
	if len(items) != 4 {
		return nil, fmt.Errorf("invalid piclib resource '%v'", path)
	}
	imgType, pName := items[2], items[3]

	p, err := c.h.lib.GetPhoto(pName)
	if err != nil {
		log.Println("pName: ", pName)
		return nil, err
	}

	var data []byte
	switch imgType {
	case MetaFile:
		data, err = json.Marshal(p)
	case OrigImg:
		data, err = c.h.lib.GetOriginal(p)
	case Thumb1Img:
		data, err = c.h.lib.GetThumb1(p)
	case Thumb2Img:
		data, err = c.h.lib.GetThumb2(p)
	default:
		return nil, fmt.Errorf("invalid image type '%v'", imgType)
	}

	if err != nil {
		return nil, err
	}
	return data, nil
}
