
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
}

function loadPageNav() {
  $("#page-nav").load("/dynamic/page-nav", slidePageNav)
}

function getNumPages() {
  $.get("/dynamic/num-pages", function(data){numPages = parseInt(data)})
}

maxDisplayPages = 25
startPage = 1

// order matters
endPage = maxDisplayPages
currPage = 0
var numPages

getNumPages()
loadPageNav()
pageTo(1)

