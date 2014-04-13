
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
  $.get("/dynamic/stat/num-pages", function(data){
    numPages = parseInt(data)
  })
  $.get("/dynamic/stat/num-pics", function(data){
    numPhotos = parseInt(data)
    $(document).keydown(keydown)
  })
}

function getCurrPic() {
  elems = window.location.href.split("/")
  return +(elems[elems.length-1])
}

var numPages = 0
var numPhotos = 0
var currPic = getCurrPic()
var keys = new Object()
keys.left = 37
keys.right = 39
keys.enter = 13

getStats()


