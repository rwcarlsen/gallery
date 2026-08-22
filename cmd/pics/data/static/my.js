async function pageTo(page) {
  if (page === currPage) {
    closeWhen()
    return
  }
  if (page < 1) {
    page = 1
  }
  if (numPages && page > numPages) {
    page = numPages
  }
  currPage = page
  const gal = document.getElementById('pic-gallery')
  gal.innerHTML = await get('/dynamic/pg/' + page)
  document.getElementById('page-nav').innerHTML = await get('/dynamic/page-nav')
  closeWhen()
}

function saveName(picid) {
  fetch('/dynamic/clickpic/' + picid)
}

function pageFirst() { pageTo(1) }
function pageLast() { pageTo(numPages) }
function pagePrev() { if (currPage > 1) pageTo(currPage - 1) }
function pageNext() { if (currPage < numPages) pageTo(currPage + 1) }

async function get(url) {
  const r = await fetch(url)
  return r.text()
}

async function loadTimeNav() {
  document.getElementById('time-nav').innerHTML = await get('/dynamic/time-nav')
}

async function getStats() {
  numPages = parseInt(await get('/dynamic/stat/num-pages'), 10) || 1
  numPhotos = parseInt(await get('/dynamic/stat/num-pics'), 10) || 0
  const el = document.getElementById('stats')
  if (el) {
    el.textContent = numPhotos + ' photos'
  }
}

async function updateNav() {
  currPage = 0
  await loadTimeNav()
  await getStats()
  const pg = parseInt(await get('/dynamic/pg'), 10) || 1
  await pageTo(pg)
}

function closeWhen() {
  const el = document.getElementById('when')
  if (el) {
    el.classList.remove('open')
  }
}

function setupWhen() {
  const wrap = document.getElementById('when')
  const btn = document.getElementById('when-btn')
  if (!btn) {
    return
  }
  btn.addEventListener('click', function (e) {
    e.stopPropagation()
    wrap.classList.toggle('open')
  })
  document.addEventListener('click', closeWhen)
}

var currPage = 0
var numPages = 0
var numPhotos = 0

document.addEventListener('keydown', function (e) {
  if (e.target && (e.target.tagName === 'TEXTAREA' || e.target.tagName === 'INPUT')) {
    return
  }
  if (e.key === 'ArrowLeft') {
    pagePrev()
  } else if (e.key === 'ArrowRight') {
    pageNext()
  }
})

setupWhen()
updateNav()
