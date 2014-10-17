
function keydown() {
  if (event.which == keys.left) {
    currPic -= 1
    if (currPic < 0) {
      currPic = 0
    }
    window.location.href = "/dynamic/zoom/" + currPic.toString()
    return false
  } else if (event.which == keys.right) {
    currPic += 1
    if (currPic >= numPhotos) {
      currPic = numPhotos - 1
    }
    window.location.href = "/dynamic/zoom/" + currPic.toString()
    return false
  }
  return true
}

function getStats() {
  $.get("/dynamic/stat/pics-per-page", function(data){
    picsPerPage = parseInt(data)
  })
  $.get("/dynamic/stat/num-pics", function(data){
    numPhotos = parseInt(data)
    $(document).keydown(keydown)

    // change thumbnail gallery page to page of current zoompic upon exiting
    // this webpage
    $(window).bind('beforeunload', function(){
      $.ajax({
        url: "/dynamic/set-page/" + Math.max(Math.ceil(currPic / picsPerPage), 1).toString(),
        async: false
      });
    });
  })
}

function getCurrPic() {
  elems = window.location.href.split("/")
  return +(elems[elems.length-1])
}

function saveNotes(index) {
  ind = index.toString()
  data = $("#pic-notes" + ind).val()
  $.post("/dynamic/save-notes/" + ind, data)
}

var picsPerPage = 0
var numPhotos = 0
var currPic = getCurrPic()
var keys = new Object()
keys.left = 37
keys.right = 39
keys.enter = 13

getStats()


