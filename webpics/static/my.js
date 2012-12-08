
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
  $.get("/dynamic/num-pages", function(data){
    numPages = parseInt(data)
    $("#num-pages").text(numPages.toString() + " pages")
  })
  $.get("/dynamic/num-pics", function(data){
    numPhotos = parseInt(data)
    $("#num-pics").text(numPhotos.toString() + " photos")
  })
}

function updateNav() {
  loadPageNav()
  loadTimeNav()
  pageTo(1)
}

// configurable
var maxDisplayPages = 25
// end configurable

// order matters
var startPage = 1
var endPage = maxDisplayPages
var currPage = 0
var numPages = 0
var numPhotos = 0

updateNav()

