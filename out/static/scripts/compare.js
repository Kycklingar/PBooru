var posts = []
var gateway = "ass"
var currentPost = null

var leftInterface = document.getElementById("interface-left")
var rightInterface = document.getElementById("interface-right")
var canvas = rightInterface.children[0]
var ctx = canvas.getContext("2d")

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

function humanFileSize(size) {
	var i = Math.floor( Math.log(size) / Math.log(1000) );
	return ( size / Math.pow(1000, i) ).toFixed(2) * 1 + ' ' + ['B', 'kB', 'MB', 'GB', 'TB'][i];
}

function addPost(post)
{
	posts.push(post)
	leftInterface.appendChild(leftPostElement(post))
}

function leftPostElement(post)
{
	let e = document.createElement("div")
	e.postid = post.id
	
	let img = document.createElement("img")
	img.src = gateway + "/ipfs/" + post.thumbnail

	img.onclick = function(){renderPost(post)}

	e.appendChild(img)

	function pEl(inner, p) {
		let el = document.createElement("p")
		el.innerText = inner
		p.appendChild(el)
	}

	pEl("ID: " + post.id, e)
	pEl(post.dimensions.Width + "x" + post.dimensions.Height, e)
	pEl(humanFileSize(post.filesize), e)
	pEl(post.mime.Type + "/" + post.mime.Name , e)

	return e
}

function ipfsLink(hash)
{
	return gateway + "/ipfs/" + hash
}

function renderPost(post)
{
	let img = new Image()
	img.src = ipfsLink(post.hash)
	img.onload = function(){renderImage(img)}
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
	scaledWidth = image.width * optScale
	scaledHeight = image.height * optScale
	ctx.clearRect(0, 0, canvas.width, canvas.height)

	if(!optAnchor || (canvas.width < scaledWidth || canvas.height < scaledHeight))
	{
		canvas.width = scaledWidth
		canvas.height = scaledHeight
	}

	canvas.style.filter = `contrast(${optContrast})`
	ctx.imageSmoothingEnabled = false
	ctx.drawImage(image, 0, 0, scaledWidth, scaledHeight)
}

function fit()
{
	if(optFit = !optFit)
	{
		canvas.style.maxWidth = "100%"
		canvas.style.maxHeight = "100%"
	} else {
		canvas.style.maxWidth = ""
		canvas.style.maxHeight = ""
	}
	document.getElementById("button-fit").classList.toggle("highlighted")
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
