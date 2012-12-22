
function pageTo(page) {
  if (page == currPage) {
    return
  }
  if (page < startPage) {
    delta = startPage - page
    startPage -= delta
    endPage -= delta
  } else if (page > endPage) {
    delta = page - endPage
    startPage += delta
    endPage += delta
  }
  slidePageNav()

  currPage = page

  setAttr = function() {
    $(".pglink").removeClass("active")
    $("#pg" + currPage.toString()).addClass("active")
    bindArrowEvents()
  }

  $("#pic-gallery").load("/dynamic/pg" + currPage.toString(), setAttr)
}

function pagePrev() {
  if (currPage > 1) {
    pageTo(currPage - 1)
  }
}

function pageNext() {
  if (currPage < numPages) {
    pageTo(currPage + 1)
  }
}

function refreshGallery() {
  pageTo(currPage)
}

function slidePageNav() {
  $(".pglink").each(function(i, elem){
    if (i+1 < startPage || i >= endPage) {
      $(this).hide()
    } else {
      $(this).show()
    }
  })
  getStats()
}

function loadPageNav() {
  $("#page-nav").load("/dynamic/page-nav", slidePageNav)
}

function loadTimeNav() {
  $("#time-nav").load("/dynamic/time-nav")
}

function getStats() {
  $.get("/dynamic/stat/num-pages", function(data){
    numPages = parseInt(data)
    $("#num-pages").text(numPages.toString() + " pages")
  })
  $.get("/dynamic/stat/num-pics", function(data){
    numPhotos = parseInt(data)
    $("#num-pics").text(numPhotos.toString() + " photos")
  })
}

function updateNav() {
  currPage = 0
  loadPageNav()
  loadTimeNav()
  $.get("/dynamic/pg", function(pg) {
    pageTo(parseInt(pg))
  })
  updateDatelessToggle()
}

function updateDatelessToggle() {
    $.post("/dynamic/stat/hiding-dateless", function(data) {
      text = "Hide Dateless"
      if (data == "true") {
        text = "Show Dateless"
      }
      $("#dateless-toggle").text(text)
    })
}

function toggleDateless() {
    $.post("/dynamic/toggle-dateless", function(data) {
      updateNav()
    })
}

function saveNotes(index) {
  ind = index.toString()
  data = $("#pic-notes" + ind).val()
  $.post("/dynamic/save-notes/" + ind, data)
}

function keydown() {
  if (mode == "zoom-view") {
  } else if (mode == "gallery-view") {
    if (event.which == keys.left) {
      pagePrev()
    } else if (event.which == keys.right) {
      pageNext()
    }
  }
}

function bindArrowEvents() {
  $(document).keydown(keydown)
  $(".zoomview").on("hide", function(ev){
    mode = "gallery-view"
  })
  $(".zoomview").on("show", function(ev){
    mode = "zoom-view"
    currZoom = ev.target.id.substring(4)
    //hide curr zoom and show next zoom
  })
}

function bindNavEvents() {
  $("#search-form").submit(function(){
    // post instead of form submit allows callback on server response
    $.post($(this).attr('action'), $(this).serialize(), function(json) {
      updateNav()
    }, 'json');
    return false; // prevent form submission and page reload
  });
}

// configurable
var maxDisplayPages = 20
// end configurable

var startPage = 1
var endPage = maxDisplayPages
var numPages = 0
var numPhotos = 0
var mode = "gallery-view"

// key codes
var keys = new Object()
keys.left = 37
keys.right = 39
keys.enter = 13

bindArrowEvents()
bindNavEvents()
updateNav()

