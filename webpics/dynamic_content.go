package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/rwcarlsen/gallery/piclib"
)

var (
	zoomTmpl    = template.Must(template.New("zoompic").Parse(zoompic))
	picsTmpl    = template.Must(template.New("browsepics").Parse(browsepics))
	pagenavTmpl = template.Must(template.New("pagination").Parse(pagination))
	timenavTmpl = template.Must(template.New("timenav").Parse(timenav))
)

const noteField = "LibNotes"

type context struct {
	allPhotos    []*piclib.Photo
	photos       []*piclib.Photo
	HideDateless bool
	CurrPage     string
	filter		 string // manual, forced pic pre-filter - trumps other filters
	query        []string
	random       []int
	randIndex    int
}

func newContext(pics []*piclib.Photo, filter string) *context {
	c := &context{
		allPhotos: pics,
		photos: pics,
		CurrPage: "1",
		filter: filter,
	}
	c.updateFilter() // initialize pic list
	return c
}

func (c *context) toggleDateless() {
	c.HideDateless = !c.HideDateless
	c.updateFilter()
}

func (c *context) setSearchFilter(query []string) {
	if c.query != nil {
		// reset if photos are already filtered
		c.query = nil
		c.updateFilter()
	}
	c.query = query
	c.updateFilter()
}

func (c *context) updateFilter() {
	c.CurrPage = "1"
	newlist := make([]*piclib.Photo, 0, len(c.photos))
	for _, p := range c.allPhotos {
		if c.passFilter(p) {
			newlist = append(newlist, p)
		}
	}
	c.photos = newlist
}

func (c *context) passFilter(p *piclib.Photo) bool {
	if c.HideDateless && !p.LegitTaken() {
		return false
	} else if !c.passesSearch(p) {
		return false
	} else if !strings.Contains(p.Tags[noteField], c.filter) {
		return false
	}
	return true
}

func (c *context) passesSearch(p *piclib.Photo) bool {
	if len(c.query) == 0 {
		return true
	}

	notes := strings.ToLower(p.Tags[noteField])
	for _, val := range c.query {
		val = strings.ToLower(val)
		for _, s := range strings.Fields(val) {
			if !strings.Contains(notes, s) {
				return false
			}
		}
	}
	return true
}

func (c *context) addPics(pics []*piclib.Photo) {
	newlist := []*piclib.Photo{}
	for _, p := range pics {
		if c.passFilter(p) {
			newlist = append(newlist, p)
		}
	}
	c.photos = append(newlist, c.photos...)
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
	p.Tags[noteField] = string(data)
	if err := lib.UpdatePhoto(p); err != nil {
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
	data, err := p.GetOriginal()
	if err != nil {
		return err
	}
	w.Write(data)
	if c.randIndex++; c.randIndex == len(c.photos) {
		c.randIndex = 0
	}
	return nil
}

func (c *context) servePage(w http.ResponseWriter, pg string) error {
	pgNum, err := strconv.Atoi(pg)
	if err != nil {
		return fmt.Errorf("invalid gallery page view request: %v", pg)
	}

	start := picsPerPage * (pgNum - 1)
	end := min(start+picsPerPage, len(c.photos))
	list := make([]*thumbData, end-start)
	for i, p := range c.photos[start:end] {
		list[i] = &thumbData{
			Path:  p.Meta,
			Date:  p.Taken.Format("Jan 2, 2006"),
			Index: i + start,
			Notes: p.Tags[noteField],
			Style: imgRotJS(p.Rotation()),
		}
	}

	if err = picsTmpl.Execute(w, list); err != nil {
		return err
	}
	c.CurrPage = pg
	return nil
}

func (c *context) serveZoom(w http.ResponseWriter, index string) error {
	i, _ := strconv.Atoi(index)
	p := c.photos[i]
	pData := &thumbData{
		Path:  p.Meta,
		Date:  p.Taken.Format("Jan 2, 2006"),
		Index: i,
		Notes: p.Tags[noteField],
		Style: imgRotJS(p.Rotation()),
	}

	return zoomTmpl.Execute(w, pData)
}

func (c *context) servePageNav(w http.ResponseWriter) error {
	n := len(c.photos)/picsPerPage + 1
	pages := make([]int, n)
	for i := range pages {
		pages[i] = i + 1
	}

	return pagenavTmpl.Execute(w, pages)
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

func (c *context) serveTimeNav(w http.ResponseWriter) error {
	if len(c.photos) == 0 {
		return nil
	}
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

	return timenavTmpl.Execute(w, years)
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

// imgRotJS returns the css3 style text required to rotate an element deg
// clockwise.
func imgRotJS(deg int) string {
	t := fmt.Sprintf("transform:rotate(%vdeg)", deg)
	//Cross-browser
	return fmt.Sprintf("-moz-%s; -webkit-%s; -ms-%s; -o-%s; %s;", t, t, t, t, t)
}

const timenav = `
{{range $i, $year := .}}
<li>
  <div class="pagination pagination-centered">
    <ul class="dropdown-wide">
      <li class="dropdown-page"><a class="dropdown-link" href="#" onclick="pageTo({{$year.StartPage}})">{{$year.Year}}</a></li>
      {{range $j, $month := $year.Months}}
      <li class="dropdown-page"><a class="dropdown-link float-left" href="#" onclick="pageTo({{$month.Page}})">{{$month.Name}}</a></li>
      {{end}}
    </ul>
  </div>
</li>
{{end}}
`
const pagination = `
<div class="pagination pagination-centered">
<ul>
  <li id="pgprev" ><a href="#" onclick="pagePrev()">Newer</a></li>
	{{range $i, $pgNum := .}}
	<li id="pg{{$pgNum}}" class="pglink"><a class="pga" href="#"
	onclick="pageTo({{$pgNum}})"><p class="pgp">{{$pgNum}}</p></a></li>
	{{end}}
  <li id="pgnext"><a href="#" onclick="pageNext()">Older</a></li>
</ul>
</div>
`
const browsepics = `
<ul class="thumb-grid group">
{{range $index, $photo := .}}
<li>
	<div style="{{$photo.Style}}">
		<a href="/dynamic/zoom/{{$photo.Index}}">
			<img class="img-rounded" src="/piclib/thumb1/{{$photo.Path}}" oncontextmenu="tagPut('{{$photo.Path}}')">
		</a>
		<div class="caption">
			<p class="pagination-centered">{{$photo.Date}}</p>
		</div>
	</div>
</li>
{{end}}
</ul>
`
const zoompic = `
<!DOCTYPE html>
<html>
	<head>
		<title>RWC Photos</title>
		<link href="/static/bootstrap/css/bootstrap.css" rel="stylesheet" media="screen">
		<link rel="shortcut icon" href="/static/favicon.ico" />

    <style>
		html, body {
			height: 100%;
			min-height: 100%;
			background-color: black;
		}
		.zoom-img {
			max-width: 80%;
			max-height: 80%;
			transform-origin:60% 60%;
			-ms-transform-origin:60% 60%; /* IE 9 */
			-webkit-transform-origin:60% 60%; /* Safari and Chrome */
			-moz-transform-origin:60% 60%; /* Firefox */
			-o-transform-origin:60% 60%; /* Opera */
		}
    </style>
	</head>

	<body>
		<div class="container black">
			<div class="row" style="text-align: center;">
				<br><br><br>
				<a href="/"><img class="zoom-img" data-dismiss="modal" src="/piclib/thumb2/{{.Path}}" style="{{.Style}}"></a>
			</div>
		</div>

		<div class="navbar navbar-fixed-bottom">
			<div class="navbar-inner">
				<div class="container">
					<ul class="nav">
						<li><textarea id="pic-notes{{.Index}}">{{.Notes}}</textarea></li>
						<li><div><a href="#" class="btn" style="margin-left: 10px" onclick="saveNotes({{.Index}})">Save Notes</a></div></li>
					</ul>
					<ul class="nav pull-right">
						<li><a href="#" disabled>Taken {{.Date}}</a></li>
						<li><div><a class="btn" href="/piclib/orig/{{.Path}}">Original</a></div></li>
					</ul>
				</div>
			</div>
		</div>

		<script src="http://code.jquery.com/jquery-latest.js"></script>
		<script type="text/javascript">
			function saveNotes(index) {
			  ind = index.toString()
			  data = $("#pic-notes" + ind).val()
			  $.post("/dynamic/save-notes/" + ind, data)
			}
		</script>

	</body>
</html>
`

