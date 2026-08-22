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

async function loadFaces() {
  const box = document.getElementById('faces')
  if (!box) {
    return
  }
  const pic = box.dataset.pic
  const edit = box.dataset.edit === '1'
  const faces = await fetch('/dynamic/faces/' + pic).then(r => r.json())
  box.innerHTML = ''
  faces.forEach(function (f) {
    const el = document.createElement('div')
    el.className = 'chip'
    const img = document.createElement('img')
    img.src = '/photo/face/' + f.id
    img.alt = f.name || 'face'
    img.addEventListener('click', async function () {
      await fetch('/dynamic/face/' + f.id + '/similar')
      window.location.href = '/'
    })
    el.appendChild(img)
    if (edit) {
      const inp = document.createElement('input')
      inp.value = f.name
      inp.placeholder = 'name'
      inp.addEventListener('keydown', async function (e) {
        if (e.key === 'Enter') {
          await fetch('/dynamic/face/' + f.id + '/name', { method: 'POST', body: inp.value })
          loadFaces()
        }
      })
      el.appendChild(inp)
    } else if (f.name) {
      const lab = document.createElement('span')
      lab.textContent = f.name
      el.appendChild(lab)
    }
    box.appendChild(el)
  })
}

async function loadMeta() {
  const el = document.getElementById('meta')
  const btn = document.getElementById('meta-btn')
  if (!el || !btn) {
    return
  }
  if (!el.hidden) {
    el.hidden = true
    return
  }
  const tags = await fetch('/dynamic/meta/' + btn.dataset.pic).then(r => r.json())
  el.innerHTML = ''
  const table = document.createElement('table')
  tags.forEach(function (t) {
    const tr = document.createElement('tr')
    const th = document.createElement('th')
    th.textContent = t.name
    const td = document.createElement('td')
    td.textContent = t.value
    tr.appendChild(th)
    tr.appendChild(td)
    table.appendChild(tr)
  })
  el.appendChild(table)
  el.hidden = false
}

var metaBtn = document.getElementById('meta-btn')
if (metaBtn) {
  metaBtn.addEventListener('click', loadMeta)
}

getStats()
loadFaces()
