
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

function loadPageNav() {
  $("#page-nav").load("/dynamic/page-nav")
}

function getNumPages() {
  $.get("/dynamic/num-pages", function(data){numPages = parseInt(data)})
}

// order matters
var numPages
currPage = 0
getNumPages()
loadPageNav()
pageTo(1)

