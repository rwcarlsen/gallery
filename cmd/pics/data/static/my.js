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
  syncWhenSlider()
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
  buildWhenSlider()
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
  const bubble = document.querySelector('.when-bubble')
  if (bubble) {
    bubble.hidden = true
  }
}

function whenStops() {
  const stops = []
  document.querySelectorAll('#time-nav .year-row').forEach(function (row) {
    const year = row.querySelector('.year')
    if (!year) {
      return
    }
    row.querySelectorAll('.month').forEach(function (a) {
      stops.push({
        page: parseInt(a.dataset.page, 10),
        label: (a.dataset.full || a.textContent) + ' ' + year.textContent
      })
    })
  })
  stops.sort(function (a, b) { return b.page - a.page })
  return stops
}

function nearestStop(stops, page) {
  let best = 0
  let bestD = Infinity
  for (let i = 0; i < stops.length; i++) {
    const d = Math.abs(stops[i].page - page)
    if (d < bestD) {
      bestD = d
      best = i
    }
  }
  return best
}

function placeWhenThumb(thumb, idx, n) {
  const t = n <= 1 ? 0.5 : idx / (n - 1)
  thumb.style.left = (t * 100) + '%'
}

function syncWhenSlider() {
  const thumb = document.querySelector('.when-thumb')
  if (!thumb) {
    return
  }
  const stops = whenStops()
  if (!stops.length) {
    return
  }
  placeWhenThumb(thumb, nearestStop(stops, currPage), stops.length)
}

function buildWhenSlider() {
  const menu = document.getElementById('time-nav')
  if (!menu) {
    return
  }
  const stops = whenStops()
  const slider = document.createElement('div')
  slider.className = 'when-slider'
  const bubble = document.createElement('div')
  bubble.className = 'when-bubble'
  bubble.hidden = true
  const track = document.createElement('div')
  track.className = 'when-track'
  const thumb = document.createElement('div')
  thumb.className = 'when-thumb'
  track.appendChild(thumb)
  slider.appendChild(bubble)
  slider.appendChild(track)
  menu.prepend(slider)
  if (!stops.length) {
    slider.hidden = true
    return
  }
  let idx = nearestStop(stops, currPage)
  placeWhenThumb(thumb, idx, stops.length)

  function setFromX(clientX) {
    const r = track.getBoundingClientRect()
    let t = 0
    if (r.width > 0) {
      t = (clientX - r.left) / r.width
    }
    if (t < 0) {
      t = 0
    }
    if (t > 1) {
      t = 1
    }
    idx = Math.round(t * (stops.length - 1))
    placeWhenThumb(thumb, idx, stops.length)
    bubble.textContent = stops[idx].label
    bubble.hidden = false
    bubble.style.left = clientX + 'px'
    bubble.style.top = r.top + 'px'
  }

  slider.addEventListener('pointerdown', function (e) {
    e.preventDefault()
    e.stopPropagation()
    slider.setPointerCapture(e.pointerId)
    setFromX(e.clientX)
  })
  slider.addEventListener('pointermove', function (e) {
    if (!slider.hasPointerCapture(e.pointerId)) {
      return
    }
    e.preventDefault()
    setFromX(e.clientX)
  })
  slider.addEventListener('pointerup', function (e) {
    if (!slider.hasPointerCapture(e.pointerId)) {
      return
    }
    e.preventDefault()
    e.stopPropagation()
    bubble.hidden = true
    pageTo(stops[idx].page)
  })
  slider.addEventListener('pointercancel', function () {
    bubble.hidden = true
  })
  slider.addEventListener('click', function (e) {
    e.stopPropagation()
  })
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
    if (wrap.classList.contains('open')) {
      syncWhenSlider()
    }
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

async function doSearch() {
  const q = document.getElementById('search-q')
  if (!q) {
    return false
  }
  await fetch('/dynamic/search?q=' + encodeURIComponent(q.value))
  currPage = 0
  await loadTimeNav()
  await getStats()
  await pageTo(1)
  return false
}

setupWhen()
updateNav()
