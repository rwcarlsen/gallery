
function pageTo(page) {
  if (page == currPage) {
    return
  }

  currPage = page

  setAttr = function() {
    $(".pglink").removeClass("active")
    $("#pg" + currPage.toString()).addClass("active")
  }

  $("#pic-gallery").load("/dynamic/pg" + currPage.toString(), setAttr)
}

function pagePrev() {
  if (currPage > 1) {
    if (currPage == startPage) {
      startPage -= 1
      endPage -= 1
      slidePageNav()
    }
    pageTo(currPage - 1)
  }
}

function pageNext() {
  if (currPage < numPages) {
    if (currPage == endPage) {
      startPage += 1
      endPage += 1
      slidePageNav()
    }
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

function getNumPhotos() {
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

loadPageNav()
pageTo(1)

