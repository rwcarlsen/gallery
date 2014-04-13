package main

import "github.com/rwcarlsen/gallery/piclib"

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
	Notes string
	Date  string
	Index int
	Style string
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
		<link href="/static/my.css" rel="stylesheet" media="screen">
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
