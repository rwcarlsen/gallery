function getCurrPic() {
  var elems = window.location.pathname.split('/')
  return parseInt(elems[elems.length - 1], 10) || 0
}

async function saveNotes(index) {
  var el = document.getElementById('pic-notes' + index)
  if (!el) {
    return
  }
  await fetch('/dynamic/save-notes/' + index, {
    method: 'POST',
    body: el.value
  })
}

var picsPerPage = 24
var numPhotos = 0
var currPic = getCurrPic()

async function getStats() {
  picsPerPage = parseInt(await fetch('/dynamic/stat/pics-per-page').then(r => r.text()), 10) || 24
  numPhotos = parseInt(await fetch('/dynamic/stat/num-pics').then(r => r.text()), 10) || 0
}

document.addEventListener('keydown', function (e) {
  if (e.target && (e.target.tagName === 'TEXTAREA' || e.target.tagName === 'INPUT')) {
    return
  }
  if (e.key === 'ArrowLeft') {
    currPic = Math.max(0, currPic - 1)
    window.location.href = '/dynamic/zoom/' + currPic
  } else if (e.key === 'ArrowRight') {
    currPic = Math.min(numPhotos - 1, currPic + 1)
    window.location.href = '/dynamic/zoom/' + currPic
  }
})

window.addEventListener('pagehide', function () {
  var pg = Math.floor(currPic / picsPerPage) + 1
  fetch('/dynamic/set-page/' + pg, { keepalive: true })
})

getStats()
