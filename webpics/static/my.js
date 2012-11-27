
function pageTo(page) {
  if (page == currPage) {
    return
  }

  currPage = page

  setAttr = function() {
    $(".pglink").removeClass("active")
    $("#pg" + currPage.toString()).addClass("active")
  }

  $("#pic-gallery").load("/pg" + currPage.toString(), setAttr)
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

currPage = 0
numPages = $(".pglink").length
pageTo(1)

