var offset = 0

document.getElementById("sizeslider").oninput = function(){
	var ss = document.getElementById("stylesheet")
	ss.innerHTML = ".thumbnail{width:" + this.value +
		"px;height:" + this.value +
		"px;}.thumbnail img{max-width:"+
		this.value +
		"px;max-height:" +
		this.value +
		"px;}"
	document.getElementById("sizeslidertext").innerText = this.value
}

function openComparePage()
{
	let f = new FormData()
	for(let i = 0; i < selectedPosts.length; i++)
	{
		f.append("post-id", selectedPosts[i].getAttribute("postid"))
	}

	let params = new URLSearchParams(f)
	console.log(params.toString())

	window.open("/compare2/?" + params.toString())

}

function updateOffset() {
	var e = document.getElementsByClassName("offsetspan")
	for(let i = 0; i < e.length; i++){
		e[i].innerText = offset
	}
}

function nextPage() {
	offset++
	updateOffset()
	let c = document.getElementById("searchinput")
	fetchPosts(c)
}

function refreshPage(){
	let c = document.getElementById("searchinput")
	fetchPosts(c)
}

function prevPage() {
	offset--
	if (offset < 0) {
		offset = 0
	}
	updateOffset()

	let c = document.getElementById("searchinput")
	fetchPosts(c)
}

function newSearch(c) {
	offset = 0
	updateOffset()
	fetchPosts(c)
}

function fetchPosts(caller) {
	var req = new XMLHttpRequest
	req.onreadystatechange = function () {
		if (this.readyState == 4 && this.status == 200) {
			try {
				generatePosts(JSON.parse(this.response))
			}
			catch (e) {
				console.log(e)
			}
		}
	}
	var fd = new FormData(caller)
	req.open("POST", "/api/v1/posts?inclTags=1&offset=" + offset, true)
	req.send(fd)
}

function pageNum(perRequest, total) {
	return Math.round((Number(total) / Number(perRequest)) + 0.5)
}

function createPost(post) {
	//var a = document.createElement("a")
	//a.href = "/post/" + post.ID + "/" + post.Hash
	var div = document.createElement("div")
	//div.id = post.ID
	div.setAttribute("postid", post.ID)
	div.className = "thumbnail " + post.Mime.split("/")[0]
	// div.style.width = UserInfo.ThumbSize
	// div.style.height = UserInfo.ThumbSize
	div.style.cursor = "pointer"
	div.title = function(){
		let ret = ""
		if(post.Tags != null)
		{
			for(let i = 0; i < post.Tags.length; i++)
			{
				if(post.Tags[i].Namespace == "none")
					ret += post.Tags[i].Tag + "\n"
				else
					ret += post.Tags[i].Namespace + ":" + post.Tags[i].Tag + "\n"
			}
		}
		return ret
	}()
	if (post.Removed == "1") {
		div.style.border = "solid red"
	}

	if (post.Thumbnails == null) {
		var d = document.createElement("div")
		d.style = "background-color:grey;"
		d.className = "centered"
		d.innerText = post.Mime
		div.appendChild(d)
	}
	else {
		thumb = closestThumb(0, post.Thumbnails)
		var img = new Image()
		img.src = UserInfo.Gateway + "/ipfs/" + thumb.Cid
		img.alt = post.Cid
		// img.style.maxWidth = UserInfo.ThumbSize
		// img.style.maxHeight = UserInfo.ThumbSize
		div.appendChild(img)
	}

	//a.appendChild(div)
	return div
}

var fetchedPosts = []

function cleanDiv(div) {
	while (div.hasChildNodes()) {
		div.removeChild(div.lastChild)
	}
}

function generatePosts(res) {
	var pdiv = document.getElementById("posts")
	cleanDiv(pdiv)

	fetchedPosts = res.Posts

	for (var i = 0; i < fetchedPosts.length; i++) {
		let p = createPost(fetchedPosts[i])
		p.setAttribute("index", i)
		p.onmousedown = function (caller) {
			if (caller.button != 0) {
				return
			}
			// console.log(caller.button)
			var t = caller.target
			while (!t.className.includes("thumbnail")) {
				t = t.parentElement
			}

			let tmp = fetchedPosts[Number(t.getAttribute("index"))]
			appendToSelectionList(constructSelectionPost(tmp))
		}

		pdiv.appendChild(p)
	}

	var pages = pageNum(res.Posts.length, res.TotalPosts)

}

function selectAll() {
	for(let i = 0; i < fetchedPosts.length; i++)
	{
		appendToSelectionList(constructSelectionPost(fetchedPosts[i]))
	}
}

function clearAll() {
	selectedPosts = []
	cleanDiv(selectionDiv)
}

function getDupInfo(postID, callback) {
	let res = new XMLHttpRequest()
	res.onreadystatechange = function () {
		if (this.readyState == 4 && this.status == 200) {
			let js = JSON.parse(this.responseText)
			callback.value = js.Level
			callback.setAttribute("dupid", js.ID)
			//console.log(js.Level)
		}
	}
	res.open("GET", "/api/v1/duplicate?id=" + postID, true)
	res.send()
}

var selectedPosts = []
var selectionDiv = document.getElementById("postlist")

function constructSelectionPost(post) {
	var div = document.createElement("div")
	div.setAttribute("postid", post.ID)
	div.className = "thumbnail " + post.Mime.split("/")[0]
	// div.style.width = UserInfo.ThumbSize
	// div.style.height = UserInfo.ThumbSize
	div.title = function(){
		let ret = ""
		if (post.Tags != null) {
			for(let i = 0; i < post.Tags.length; i++)
			{
				ret += post.Tags[i].Namespace + ":" + post.Tags[i].Tag + "\n"
			}
		}
		return ret
	}()

	if (post.Thumbnails == null) {
		var d = document.createElement("div")
		d.style = "background-color:grey;"
		d.className = "centered"
		d.innerText = post.Mime
		let a = document.createElement("a")
		a.href = "/post/" + post.ID
		a.target = "_blank"
		a.appendChild(d)
		div.appendChild(a)
	}
	else {
		thumb = closestThumb(0, post.Thumbnails)
		var img = new Image()
		img.src = UserInfo.Gateway + "/ipfs/" + thumb.Cid
		img.alt = post.Cid
		// img.style.maxWidth = UserInfo.ThumbSize
		// img.style.maxHeight = UserInfo.ThumbSize
		let a = document.createElement("a")
		a.href = "/post/" + post.ID
		a.target = "_blank"
		a.appendChild(img)
		div.appendChild(a)
	}
	//a.appendChild(div)

	let xdiv = document.createElement("div")
	xdiv.style.position = "absolute"
	xdiv.style.right = "0px"
	xdiv.style.top = "0px"
	xdiv.style.backgroundColor = "#832"
	xdiv.innerText = "X"
	xdiv.style.cursor = "pointer"

	xdiv.onmousedown = function (c) {
		var t = c.target
		while (!t.className.includes("thumbContainer")) {
			t = t.parentElement
		}
		removeFromSelectionList(t)
	}
	div.appendChild(xdiv)

	let sdiv = document.createElement("div")
	sdiv.style.position = "absolute"
	sdiv.style.left = "0px"
	sdiv.style.top = "0px"
	sdiv.style.backgroundColor = "#832"
	sdiv.innerText = "\\"
	sdiv.style.cursor = "pointer"

	sdiv.onmousedown = function (c) {
		var t = c.target
		while (!t.className.includes("thumbContainer")) {
			t = t.parentElement
		}
		gotoSimilar(t)
	}
	div.appendChild(sdiv)

	let ldiv = document.createElement("div")
	ldiv.style.position = "absolute"
	ldiv.style.left = "0px"
	ldiv.style.top = "40px"
	ldiv.style.bottom = "0px"
	ldiv.style.width = "20px"
	ldiv.style.backgroundColor = "#222"
	ldiv.innerText = "<"
	ldiv.style.cursor = "pointer"

	ldiv.onmousedown = function (c) {
		var t = c.target
		while (!t.className.includes("thumbContainer")) {
			t = t.parentElement
		}
		moveInSelection(t, "left")
	}
	div.appendChild(ldiv)

	let rdiv = document.createElement("div")
	rdiv.style.position = "absolute"
	rdiv.style.right = "0px"
	rdiv.style.top = "40px"
	rdiv.style.bottom = "0px"
	rdiv.style.width = "20px"
	rdiv.style.backgroundColor = "#222"
	rdiv.innerText = ">"
	rdiv.style.cursor = "pointer"

	rdiv.onmousedown = function (c) {
		var t = c.target
		while (!t.className.includes("thumbContainer")) {
			t = t.parentElement
		}
		moveInSelection(t, "right")
	}
	div.appendChild(rdiv)

	let e = document.createElement("div")
	e.setAttribute("postid", div.getAttribute("postid"))
	e.className = "thumbContainer"
	let label = document.createElement("p")
	label.innerText = post.ID
	let inp = document.createElement("input")
	inp.type = "text"
	//getDupInfo(e.getAttribute("postid"), inp)

	e.appendChild(div)
	e.appendChild(label)
	e.appendChild(inp)
	return e
}

function appendToSelectionList(e) {
	for (var i = 0; i < selectedPosts.length; i++) {
		//console.log(selectedPosts[i], e.getAttribute("postid"))
		if (selectedPosts[i].getAttribute("postid") == e.getAttribute("postid")) {
			return
		}
	}

	selectionDiv.appendChild(e)
	selectedPosts.push(e)
}

function removeFromSelectionList(child) {
	document.getElementById("postlist").removeChild(child)
	for (var i = 0; i < selectedPosts.length; i++) {
		if (selectedPosts[i].getAttribute("postid") == child.getAttribute("postid")) {
			selectedPosts.splice(i, 1)
			break
		}
	}
}

function moveInSelection(child, direction) {
	if (selectedPosts.length <= 1) {
		return
	}
	var i = 0
	for (; i < selectedPosts.length; i++) {
		if (selectedPosts[i] == child) {
			break
		}
	}

	switch (direction) {
		case "left":
			{
				if (i <= 0) {
					return
				}
				let tmp = selectedPosts[i - 1]
				selectedPosts[i - 1] = child
				selectedPosts[i] = tmp
			}
			break
		case "right":
			{
				if (i >= selectedPosts.length - 1) {
					return
				}
				let tmp = selectedPosts[i + 1]
				selectedPosts[i + 1] = child
				selectedPosts[i] = tmp
			}
			break
		default:
			return
	}

	cleanDiv(selectionDiv)
	for (i = 0; i < selectedPosts.length; i++) {
		selectionDiv.appendChild(selectedPosts[i])
	}
}

function removePost() 
{
	function d(posts, index)
	{
		if(index >= posts.length)
		{
			alert("Finished")
			return
		}
		let req = new XMLHttpRequest()
		req.onreadystatechange = function(){
			if(this.readyState == 4)
			{
				if(this.status != 200)
				{
					console.log(this.status, this.responseText)
					alert("Error, check log")
					return
				} else
				{
					console.log(this.status)
				}
				posts[index].style.border = "solid 1px red"
				d(posts, index + 1)
			}
		}
		let postID = posts[index].getAttribute("postid")
		console.log("working on postid ", postID)
		let fd = new FormData()
		fd.append("post-id", postID)
		req.open("POST", "/post/edit/remove/", true)
		req.send(fd)
	}

	d(selectedPosts, 0)
}

function assignAlts()
{
	let req = new XMLHttpRequest()
	req.onreadystatechange = function() {
		if (this.readyState == 4)
		{
			if(this.status != 200)
			{
				console.log(this.status, this.responseText)
				alert("Error, check log")
				return
			} else
			{
				alert("Finished:", this.status)
			}
		}
	}

	//let fd = new FormData()
	let q = "?"

	for (let i = 0; i < selectedPosts.length; i++)
	{
		q += "post-id=" + selectedPosts[i].getAttribute("postid") + "&"
		//console.log(selectedPosts[i])
		//fd.append("post-id", selectedPosts[i].getAttribute("postid"))
	}

	//console.log(fd)

	req.open("POST", "/post/edit/alts/assign/" + q, true)
	req.setRequestHeader("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8");
	req.send()
}

function editTags(caller){
	//function t(form, arr, index){
	//	if(index >= arr.length)
	//	{
	//		alert("Finished")
	//		return
	//	}

	//	let req = new XMLHttpRequest()
	//	req.onreadystatechange = function(){
	//		if(this.readyState == 4){
	//			if(this.status != 200){
	//				console.log(this.status, this.responseText)
	//			}else{
	//				console.log(this.status)
	//			}
	//			t(form, arr, index + 1)
	//		}
	//	}
	//	console.log(arr[index].getAttribute("postid"))
	//	let fd = new FormData(form)
	//	fd.set("post-id", arr[index].getAttribute("postid"))
	//	req.open("POST", form.action, true)
	//	req.send(fd)
	//}

	//t(caller, selectedPosts, 0)
	
	let req = new XMLHttpRequest()
	req.onreadystatechange = function(){
		if(this.readyState == 4)
		{
			if(this.status != 200)
				console.log(this.status, this.responseText)
			alert("Finished " + this.status)
		}
	}

	let fd = new FormData(caller)
	for(let i = 0; i < selectedPosts.length; i++)
	{
		fd.append("pid", selectedPosts[i].getAttribute("postid"))
	}

	console.log(fd)

	req.open("POST", caller.action, true)
	req.send(fd)
}

document.getElementById("posts").onmousedown = function (caller) {
	var t = caller.target
	//console.log(t)
	if (t.getAttribute("postid") == null) {
		t = t.parentElement
		//console.log(t)                
		if (t.getAttribute("postid") == null) {
			//console.log(t)
			document.getElementById("postmenu").setAttribute("postid", null)
			return
		}
	}
	//console.log(t.getAttribute("postid"))
	document.getElementById("postmenu").setAttribute("postid", t.getAttribute("postid"))
}

function gotoSimilar(caller) {
	console.log("similar")
	let id = caller.getAttribute("postid")
	if (id == 0 || id == "null") {
		return
	}
	fetchSimilarPosts(id)
}

function fetchSimilarPosts(id) {
	var req = new XMLHttpRequest
	req.onreadystatechange = function () {
		if (this.readyState == 4 && this.status == 200) {
			try {
				generatePosts(JSON.parse(this.response))
			}
			catch (e) {
				console.log(e)
			}
		}
	}
	req.open("POST", "/api/v1/similar?id=" + id, true)
	req.send()
}

function closestThumb(size, thumbnails)
{
	let thumb = null
	for(let i = 0; i < thumbnails.length; i++)
	{
		if(thumb == null || thumbnails[i].Size > thumb.Size)
		{
			thumb = thumbnails[i]
		}
	}
	return thumb
}

function postComicPages(){
	let chapterID = document.getElementById("chapteridinput").value

	if(Number(chapterID) == 0)
	{
		console.log("Want: chapterID. Got:", chapterID)
		return
	}

	function getInput(e) {
		//console.log("getInput->", e)
		for (let i = 0; i < e.children.length; i++) {
			if (e.children[i].tagName == "INPUT") {
				return e.children[i]
			}
		}
		return null
	}

	for(let i = 0; i < selectedPosts.length; i++){
		if(getInput(selectedPosts[i]).value == "")
		{
			console.log("post have no page number")
			return
		}
	}

	function t(arr, index, chapterID) {
		if (index >= arr.length) {
			alert("Finished")
			return
		}

		let postID = arr[index].getAttribute("postid")
		if (postID == null || postID == 0) {
			console.log("bad postID ", postID)
			return
		}

		let inp = getInput(arr[index])
		if (inp == null) {
			console.log("inp is null", getInput)
			return
		}

		let page = inp.value

		if (page == null || page == "") {
			console.log("page is null: ", arr[index])
			return
		}

		let req = new XMLHttpRequest()
		req.onreadystatechange = function () {
			if (this.readyState == 4) {
				console.log(this.status)
				if(this.status == 200){
					t(arr, index + 1, chapterID)
				}
				else{
					console.log(arr[index])
					alert("Something went wrong")
				}
			}
		}
		req.open("POST", "/comic/page/add/?chapter-id=" + chapterID + "&post-id=" + postID + "&order=" + page, true)
		req.send()
	}

	t(selectedPosts, 0, chapterID)
}
