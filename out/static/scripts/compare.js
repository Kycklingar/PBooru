var posts = []
var removed = []
var gateway = "ass"
var currentPost = null
var preload = {}

var reportID = null

var leftInterface = document.getElementById("interface-left")
var rightInterface = document.getElementById("interface-right")
var canvas = document.getElementById("canvas")

var note = document.getElementById("note")
var reportNoneDupes = document.getElementById("non-dupes")

canvas.onclick = function(){rightInterface.focus()}

function postStruct(id, cid, thumb, preview, dimensions, filesize, mime, removed)
{
	return {
		"id":id,
		"cid":cid,
		"thumbnail":thumb,
		"preview":preview,
		"dimensions":dimensions,
		"filesize":filesize,
		"mime":mime,
		"removed":removed,
	}
}

function preloadImage(post)
{
	if(preload[post.id] == null)
	{
		let img = new Image()
		preload[post.id] = img

		if (optElim)
			img.src = ipfsLink(post.preview)
		else
			img.src = ipfsLink(post.cid)
	}
}

function cancelPreload(id)
{
	if(preload[id] == null)
		return

	preload[id].src = ""
	delete preload[id]
}

function cancelPreloads()
{
	let keys = Object.keys(preload)
	for(let i = 0; i < keys.length; i++)
	{
		cancelPreload(keys[i])
	}
}

function assignAlts()
{
	if(posts.length <= 1) return

	let formdata = new FormData()
	posts.forEach(p => formdata.append("post-id", p.id))

	fetch("/post/edit/alts/assign/", {
		method: "POST",
		body: formdata,
	})
	.then(response => {
		if(!response.ok)
			throw new Error(`HTTP error! Status: ${response.status}`)
		alert("Assignment complete!")
	})
	.catch(alert)
}

function submitReport()
{
	if (currentPost == null)
	{
		alert("No post selected as superior")
		return
	}

	if (!reportNoneDupes.checked && posts.length <= 1)
	{
		alert("Need more than 1 post")
		return
	}

	let fd = new FormData()
	for (let i = 0; i < posts.length; i++)
	{
		fd.append("post-ids", posts[i].id)
	}

	if (reportNoneDupes.checked)
		fd.append("non-dupes", "on")

	for (let i = 0; i < removed.length; i++)
	{
		fd.append("removed-ids", removed[i].id)
	}

	fd.append("best-id", currentPost.id)

	fd.append("note", note.value)

	if (reportID != null)
		fd.append("report-id", reportID)

	let xhr = new XMLHttpRequest()

	xhr.onreadystatechange = function() {
		if (this.readyState == XMLHttpRequest.DONE)
		{
			if (this.status == 200)
			{
				alert("Thank you for your report")
			}
			else
			{
				alert("Error: " + this.status + " " + this.responseText)
			}
		}
	}
	xhr.open("POST", "/duplicate/report/", true)

	xhr.send(fd)
}

function humanFileSize(size) {
	var i = Math.floor( Math.log(size) / Math.log(1000) );
	return ( size / Math.pow(1000, i) ).toFixed(2) * 1 + ' ' + ['B', 'kB', 'MB', 'GB', 'TB'][i];
}

function addPost(post)
{
	for(let i = 0; i < posts.length; i++)
	{
		if (posts[i].id == post.id)
			return
	}

	posts.push(post)
	leftInterface.appendChild(leftPostElement(post))
	preloadImage(post)
}

function closestThumb(minsize, thumbs)
{
	let r = null
	for(let i = 0; i < thumbs.length; i++)
	{
		if (
			r == null || (
				thumbs[i].Size > r.Size && r < minsize
			) || thumbs[i].Size < r.Size
		)
			r = thumbs[i]
	}

	return r.Cid
}

function mimeObj(mime)
{
	let s = mime.split("/")
	return {"Type":s[0],"Name":s[1]}
}

function getRemotePost(id)
{
	let xhr = new XMLHttpRequest()

	xhr.onreadystatechange = function() {
		if(xhr.readyState == XMLHttpRequest.DONE)
		{
			if(xhr.status == 200)
			{
				let j = JSON.parse(xhr.responseText)
				addPost(
					postStruct(
						j.ID,
						j.Cid,
						closestThumb(100, j.Thumbnails),
						closestThumb(1024, j.Thumbnails),
						j.Dimension,
						j.Filesize,
						mimeObj(j.Mime),
						j.Removed
					)
				)
			}
			else
			{
				alert(xhr.responseText)
			}
			
		}
	}

	let fd = new FormData()
	fd.append("id", id)

	xhr.open("POST", "/api/v1/post", true)
	xhr.send(fd)
}

function removeCurrentPost()
{
	if(currentPost != null)
	{
		removePost(currentPost.id)
	}
}

function removePost(id)
{
	// Cancel preloading image
	cancelPreload(id)

	// Cancel currently loading image
	niload()

	// If the current post is removed, render previous
	// Do not render if its the last post
	if (currentPost != null && currentPost.id == id && posts.length > 1)
	{
		// Render previous post unless at the top
		currentPost == posts[0] ? renderNextPost(1) : renderNextPost(-1)
	} else if (currentPost != null && currentPost.id != id) {
		// Do nothing
	} else {
		blankCanvas()
	}

	// Splice posts array
	for (let i = 0; i < posts.length; i++)
	{
		if (posts[i].id == id)
		{
			removed = removed.concat(posts.splice(i, 1))
			break
		}
	}

	// Remove post elements
	for (let c = leftInterface.firstChild; c != null; c = c.nextSibling)
	{
		if (c.postid == id)
		{
			leftInterface.removeChild(c)
			break
		}
	}
}

function restoreLastRemoved()
{
	if (removed.length <= 0)
	{
		return
	}

	addPost(removed.pop())
}

function leftPostElement(post)
{
	let e = document.createElement("div")
	e.postid = post.id
	e.style.position = "relative"
	e.draggable = true
	e.ondragstart = drag
	//e.ondrop = drop
	e.ondragover = dragover

	if(post.removed)
		e.classList.add("removed")

	e.addEventListener("drop", drop, true)
	
	let img = document.createElement("img")
	img.src = gateway + "/ipfs/" + post.thumbnail

	img.onclick = function(){renderPost(post)}
	img.draggable = false

	e.appendChild(img)

	function pEl(inner, p) {
		let el = document.createElement("p")
		el.innerText = inner
		p.appendChild(el)
	}

	let ide = document.createElement("p")
	ide.innerText = "ID: "
	let pa = document.createElement("a")
	pa.innerText = post.id
	pa.href = "/post/" + post.id
	pa.target = "_blank"
	ide.appendChild(pa)
	e.appendChild(ide)

	if (!(post.dimensions.Width == 0 || post.dimensions.Height == 0))
		pEl(post.dimensions.Width + "x" + post.dimensions.Height, e)
	else
		pEl("Dimensions unknown", e)

	pEl(humanFileSize(post.filesize), e)
	pEl(post.mime.Type + "/" + post.mime.Name , e)

	// Remove button
	let x = document.createElement("span")
	x.innerText = "x"
	x.className = "x"
	x.onclick = function(){removePost(e.postid)}

	e.appendChild(x)

	return e
}

function drag(e)
{
	e.dataTransfer.setData("text/plain", e.target.postid)
}

function drop(e)
{
	e.preventDefault()

	let target = null
	for(let p = e.target; p != null; p = p.parentElement)
	{
		if(p.draggable)
		{
			target = p
			break
		}
	}

	let computed = target.offsetHeight + target.offsetTop - target.scrollTop - e.clientY - target.offsetHeight / 2
	let postid = e.dataTransfer.getData("text")
	for (let c = leftInterface.firstChild; c != null; c = c.nextSibling)
	{
		if (c.postid == postid)
		{
			if(computed > 0)
				leftInterface.insertBefore(c, target)
			else
				leftInterface.insertBefore(c, target.nextSibling)
			break
		}
	}


	reorderPostsByElements()
}

function dragover(e)
{
	e.preventDefault()
}

function reorderPostsByElements()
{
	let arr = []
	for (let c = leftInterface.firstChild; c != null; c = c.nextSibling)
	{
		for (let i = 0; i < posts.length; i++)
		{
			if (c.postid == posts[i].id)
			{
				arr.push(posts[i])
			}
		}
	}

	posts = arr
}

function ipfsLink(cid)
{
	return gateway + "/ipfs/" + cid
}

let lt = null
let iload = null

function niload()
{
	if (iload)
	{
		iload.onload = null
		iload.src = ""
		iload = null
	}
}

function renderPost(post)
{
	var a = rightInterface.scrollTop
	var b = rightInterface.scrollHeight - rightInterface.clientHeight
	if (b)
		scrollY = a / b
	a = rightInterface.scrollLeft
	b = rightInterface.scrollWidth - rightInterface.clientWidth
	if (b)
		scrollX = a / b

	if (post == null)
	{
		//ctx.clearRect(0, 0, canvas.width, canvas.height)
		return
	}

	if (lt)
		clearTimeout(lt)
	lt = setTimeout(function(){loader.classList.remove("hidden")}, 250)

	// Remove any attemting-to-load images
	niload()

	let img = new Image()
	iload = img

	let src = ipfsLink(post.cid)
	if (optElim)
	{
		src = ipfsLink(post.preview)
	}

	img.src = src
	img.onload = function(){
		img.decode().then(function(){
			clearTimeout(lt)
			loader.classList.add("hidden")
			renderImage(img)
		})
	}

	currentPost = post


	for (c = leftInterface.firstChild; c != null; c = c.nextSibling)
	{
		if(c.postid == post.id)
		{
			c.classList.add("highlighted")
			c.scrollIntoView({behavior:"smooth", block:"nearest"})
		} else {
			c.classList.remove("highlighted")
		}
	}

	rightInterface.focus()
}

function renderNextPost(dir)
{
	if (currentPost == null)
	{
		renderPost(posts[0])
		return
	}

	let ind = 0
	for (let i = 0; i < posts.length; i++)
	{
		if(posts[i] == currentPost)
		{
			ind = i + dir 
			break
		}
	}

	ind = ind < 0 ? posts.length -1 : ind
	renderPost(posts[ind % posts.length])
}

var optScale = 1
var optContrast = 1
var optFit = false
var optGlue = false
var optElim = false

var gluedW, gluedH

var scrollX = 0.5
var scrollY = 0.5

function renderImage(image)
{
	var scaledWidth = image.width * optScale
	var scaledHeight = image.height * optScale

	image.style.width = scaledWidth
	image.style.height = scaledHeight

	if (optFit)
	{
		image.style.width = null
		image.style.height = null
	}
	else if (optGlue)
	{
		if (gluedW > 0 && gluedH > 0)
		{
			image.style.width = gluedW
			image.style.height = gluedH
		}
		else
		{
			gluedW = scaledWidth
			gluedH = scaledHeight
		}
	}
	else
	{
		gluedW = 0
		gluedH = 0
	}


	canvas.style.filter = `contrast(${optContrast})`
	
	if (iload == image)
	{
		iload = null
		blankCanvas()
		canvas.appendChild(image)
	}

	var b = rightInterface.scrollHeight - rightInterface.clientHeight
	rightInterface.scrollTop = b * scrollY

	b = rightInterface.scrollWidth - rightInterface.clientWidth
	rightInterface.scrollLeft = b * scrollX
}

function blankCanvas()
{
	for (let c = canvas.firstChild; c != null; c = canvas.firstChild)
		canvas.removeChild(c)
}

function toggleEliminationMode()
{
	optElim = !optElim

	cancelPreloads()

	for (let i = 0; i < posts.length; i++)
	{
		preloadImage(posts[i])
	}

	document.getElementById("button-elim").classList.toggle("highlighted")
	document.getElementById("elimination-warning").classList.toggle("hidden")
	renderPost(currentPost)
}

function toggleAltsTab()
{
	let at = document.getElementById("alts-tab")
	at.open = !at.open
	at.parentElement.open = at.open

	if(at.open) at.querySelector("button").focus()
	else rightInterface.focus()
}

function toggleReport()
{
	let rt = document.getElementById("report-tab")
	rt.open = !rt.open

	rt.parentElement.open = rt.open

	if (rt.open)
		setTimeout(function(){note.focus()}, 100)
	else
		rightInterface.focus()
}

function fit()
{
	optFit = !optFit
	canvas.classList.toggle("fit-image")
	document.getElementById("button-fit").classList.toggle("highlighted")
	renderPost(currentPost)
}

function cb(p)
{
	for(let i = 0; i < p.children.length; i++)
	{
		p.children[i].classList.remove("highlighted")
	}
}

function tb(caller)
{
	caller.classList.toggle("highlighted")
}

//function anchor()
//{
//	document.getElementById("button-anchor").classList.toggle("highlighted")
//	optAnchor = !optAnchor
//	renderPost(currentPost)
//}

function glue()
{
	document.getElementById("button-glue").classList.toggle("highlighted")
	optGlue= !optGlue
	renderPost(currentPost)
}

function scale(val)
{
	gluedW = 0
	gluedH = 0
	optScale = val
	renderPost(currentPost)

	cb(document.getElementById("scale"))
	document.getElementById(`button-scale${val}`).classList.add("highlighted")
}

function cont(val)
{
	optContrast = val
	renderPost(currentPost)
	cb(document.getElementById("contrast"))
	document.getElementById(`button-cont${val}`).classList.add("highlighted")
}

let keymap = {}
let tmpKeymap = null

const enableKeymap       = ()              => keymap = tmpKeymap
const disableKeymap      = ()              => [tmpKeymap, keymap] = [keymap, {}]
const registerKeyMapping = (key, callback) => keymap[key] = callback
const processKey         = (e)             => keymap[e.key]?.(e)

const motion        = (x, y)          => rightInterface.scrollBy({top: y, left: x, behavior: "smooth"})
const motionUp      = (scale)         => motion(0, scale * -1)
const motionRight   = (scale)         => motion(scale * 1, 0)
const motionDown    = (scale)         => motion(0, scale * 1)
const motionLeft    = (scale)         => motion(scale * -1, 0)
const shiftMotion   = (motion, shift) => (scale) => motion(scale * shift)
const processMotion = (motion, scale) => (e) => motion(scale)

const motionScale      = 100
const motionShiftScale = 10

document.onkeydown = processKey

// Motions
registerKeyMapping('h', processMotion(motionLeft, motionScale))
registerKeyMapping('j', processMotion(motionDown, motionScale))
registerKeyMapping('k', processMotion(motionUp, motionScale))
registerKeyMapping('l', processMotion(motionRight, motionScale))

registerKeyMapping('H', processMotion(shiftMotion(motionLeft, motionShiftScale), motionScale))
registerKeyMapping('J', processMotion(shiftMotion(motionDown, motionShiftScale), motionScale))
registerKeyMapping('K', processMotion(shiftMotion(motionUp, motionShiftScale), motionScale))
registerKeyMapping('L', processMotion(shiftMotion(motionRight, motionShiftScale), motionScale))

// Open report tab
registerKeyMapping('r', function(){toggleReport()})

// Open alts tab
registerKeyMapping('a', toggleAltsTab)

// Next, previous post
registerKeyMapping('n', function(){renderNextPost(1)})
registerKeyMapping('p', function(){renderNextPost(-1)})

// Remove post
registerKeyMapping('y', function(){removeCurrentPost()})

// Restore removed
registerKeyMapping('u', function(){restoreLastRemoved()})

// Glue
registerKeyMapping('q', function(){glue()})

// Fit
registerKeyMapping('f', function(){fit()})

// Elimination mode
registerKeyMapping('t', function(){toggleEliminationMode()})

// Scale
registerKeyMapping('s', function(){
	registerKeyMapping('1', function(){
		scale(1)
	})
	registerKeyMapping('2', function(){
		scale(2)
	})
	registerKeyMapping('3', function(){
		scale(5)
	})
	registerKeyMapping('4', function(){
		scale(10)
	})
})

// Contrast
registerKeyMapping('c', function(){
	
	registerKeyMapping('1', function(){
		cont(1)
	})
	registerKeyMapping('2', function(){
		cont(2)
	})
	registerKeyMapping('3', function(){
		cont(3)
	})
})
