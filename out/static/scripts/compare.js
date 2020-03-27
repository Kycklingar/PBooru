var posts = []
var gateway = "ass"
var currentPost = null

var reportID = null

var leftInterface = document.getElementById("interface-left")
var rightInterface = document.getElementById("interface-right")
var canvas = document.getElementById("canvas")

var note = document.getElementById("note")

canvas.onclick = function(){rightInterface.focus()}

function postStruct(id, hash, thumb, dimensions, filesize, mime)
{
	return {
		"id":id,
		"hash":hash,
		"thumbnail":thumb,
		"dimensions":dimensions,
		"filesize":filesize,
		"mime":mime,
	}
}

function preloadImage(post)
{
	new Image().src = gateway + "/ipfs/" + post.hash
}

function submitReport()
{
	if (currentPost == null)
	{
		alert("No post selected as superior")
		return
	}

	if (posts.length <= 1)
	{
		alert("Need more than 1 post")
		return
	}

	let fd = new FormData()
	for (let i = 0; i < posts.length; i++)
	{
		fd.append("post-ids", posts[i].id)
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

	return r.Hash
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
						j.Hash,
						closestThumb(100, j.ThumbHashes),
						j.Dimension,
						j.Filesize,
						mimeObj(j.Mime)
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

function removePost(id)
{
	for (let i = 0; i < posts.length; i++)
	{
		if (posts[i].id == id)
		{
			posts.splice(i, 1)
			break
		}
	}

	for (let c = leftInterface.firstChild; c != null; c = c.nextSibling)
	{
		if (c.postid == id)
		{
			leftInterface.removeChild(c)
			break
		}
	}

	if (currentPost.id == id)
	{
		renderNextPost(1)
	}
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

	pEl("ID: " + post.id, e)
	if (post.dimensions != null)
		pEl(post.dimensions.Width + "x" + post.dimensions.Height, e)
	else
		pEl("Dimensions unknown", e)
	pEl(humanFileSize(post.filesize), e)
	pEl(post.mime.Type + "/" + post.mime.Name , e)

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

function ipfsLink(hash)
{
	return gateway + "/ipfs/" + hash
}

let lt = null

function renderPost(post)
{
	if (post == null)
	{
		//ctx.clearRect(0, 0, canvas.width, canvas.height)
		return
	}

	if (lt)
		clearTimeout(lt)
	lt = setTimeout(function(){loader.classList.remove("hidden")}, 1000)

	let img = new Image()
	img.src = ipfsLink(post.hash)
	img.onload = function(){
		clearTimeout(lt)
		loader.classList.add("hidden")
		renderImage(img)
	}

	currentPost = post


	for (c = leftInterface.firstChild; c != null; c = c.nextSibling)
	{
		if(c.postid == post.id)
		{
			c.classList.add("highlighted")
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

var optAnchor = false
var optScale = 1
var optContrast = 1
var optFit = false

function renderImage(image)
{
	var scaledWidth = image.width * optScale
	var scaledHeight = image.height * optScale

	image.style.width = scaledWidth
	image.style.height = scaledHeight

	if (optFit)
	{
		canvas.style.width = null
		canvas.style.height = null
		image.style.width = null
		image.style.height = null
	}
	else if (optAnchor)
	{
		let rect = canvas.getBoundingClientRect()
		if(rect.width < scaledWidth)
		{
			canvas.style.width = scaledWidth
		}
		if(rect.height < scaledHeight)
		{
			canvas.style.height = scaledHeight
		}
	}
	else
	{
		canvas.style.width = scaledWidth
		canvas.style.height = scaledHeight
	}


	canvas.style.filter = `contrast(${optContrast})`
	
	for (let c = canvas.firstChild; c != null; c = canvas.firstChild)
		canvas.removeChild(c)

	canvas.appendChild(image)
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

function anchor()
{
	document.getElementById("button-anchor").classList.toggle("highlighted")
	optAnchor = !optAnchor
	renderPost(currentPost)
}

function scale(val)
{
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

document.onkeydown = processKey

var keymap = [
]

var tmpKeymap = []

function enableKeymap()
{
	keymap = tmpKeymap
}

function disableKeymap()
{
	tmpKeymap = keymap
	keymap = []
}

function toggleKeymap()
{
	if (keymap.length > 0 )
	{
		tmpKeymap = keymap
		keymap = []
	}
	else
	{
		keymap = tmpKeymap
	}
}

function registerKeyMapping(keycode, callback)
{
	let obj = {
		"v":keycode,
		"f":callback
	}
	for(let i = 0; i < keymap.length; i++)
	{
		if(keymap[i].v == keycode)
		{
			keymap[i] = obj
			return
		}
	}
	
	keymap.push(obj)
}

function processKey(e)
{
	for (let i = 0; i < keymap.length; i++)
	{
		if (keymap[i].v == e.keyCode)
		{
			keymap[i].f()
		}
	}
}

// Open report tab
registerKeyMapping(82, function(){toggleReport()})

// Next, previous post
registerKeyMapping(78, function(){renderNextPost(1)})
registerKeyMapping(80, function(){renderNextPost(-1)})

// Anchor
registerKeyMapping(65, function(){anchor()})

// Fit
registerKeyMapping(70, function(){fit()})

// Scale
registerKeyMapping(83, function(){
	registerKeyMapping(49, function(){
		scale(1)	
	})
	registerKeyMapping(50, function(){
		scale(2)	
	})
	registerKeyMapping(51, function(){
		scale(5)	
	})
})

// Contrast
registerKeyMapping(67, function(){
	
	registerKeyMapping(49, function(){
		cont(1)	
	})
	registerKeyMapping(50, function(){
		cont(2)	
	})
	registerKeyMapping(51, function(){
		cont(3)	
	})
})
