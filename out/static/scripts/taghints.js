document.addEventListener("click", removeSTBox)

document.addEventListener("DOMContentLoaded", function(){
	document.querySelectorAll(".tag-input").forEach((inputElement)=>{
		inputElement.addEventListener("input", setSTBoxTimeout)
		inputElement.autocomplete = "off"
	})
})

let timeout

function setSTBoxTimeout(ev) {
	clearTimeout(timeout)
	timeout = setTimeout(suggestionBox, 200, ev.target)
}

function removeSTBox() {
	clearTimeout(timeout)
	var st = document.getElementById("stbox")
	if (st == null) {
		return
	}
	st.parentElement.removeChild(st)
	st.removeEventListener("keydown", keyDown)
	st = null
}

function boxTimestamp() {
	var st = document.getElementById("stbox")
	if (st == null)
		return null
	return st.timestamp
}

function suggestionBox(inputElement)
{
	let tag = getTagAtCursor(inputElement)
	if(tag.length < 3) {
		removeSTBox()
		return
	}

	inputElement.addEventListener("keydown", keyDown)
	let requestTimestamp = Date.now()

	let params = new URLSearchParams()
	params.append("tags", escapeTag(tag))
	let req = new Request(`/api/v1/suggesttags?${params.toString()}`)
	fetch(req).then((response) => {
		if(response.status != 200)
			return
		return response.json()
	}).then((tags) => {
		removeSTBox()
		if(!tags)return
		constructSTBox(inputElement, tags, requestTimestamp)
	}).catch((error) => {
		console.error(error)
	})
}

function constructSTBox(inputElement, tags, timestamp)
{
	let stb = document.createElement("div")
	stb.id = "stbox"
	stb.timestamp = timestamp
	inputElement.parentElement.insertBefore(stb, inputElement.nextSibling)

	let n = Math.min(tags.length, 5)
	for(let i = 0; i < n; i++) {
		let tag = tags[i]

		let sp = document.createElement("div")
		sp.style.fontSize = "16px"
		sp.style.cursor = "pointer"
		sp.className = "tag namespace-" + tag.Namespace

		switch(tag.Namespace) {
			case "none":
				sp.innerText = tag.Tag
				break
			default:
				sp.innerText = `${tag.Namespace}:${tag.Tag}`
		}

		sp.onclick = function() {
			replaceInputTag(inputElement, sp.innerText)
			while(stb.children.length > 0)
				stb.removeChild(stb.children[0])
			inputElement.focus()
			removeSTBox()
		}
		
		stb.appendChild(sp)
	}
}

function updateSelection(box)
{
	let n = box.children.length + 1
	box.selection = (box.selection + n + 1) % n - 1

	for(let i = 0; i < box.children.length; i++)
		box.children[i].style.outline = "none"

	if(box.selection < 0)
		return
	box.children[box.selection].style.outline = "dotted black"
}

function keyDown(e){
	let box = document.getElementById("stbox")
	if(box == null){
		return
	}
	if(box.selection == null)
	{
		box.selection = -1
	}

	switch(e.key) {
		case "Escape": removeSTBox(); break
		case "Tab":
			e.preventDefault()
			if(e.shiftKey) box.selection--
			else box.selection++
			updateSelection(box)
			break
		case "Enter":
			removeSTBox()
			if(box.selection < 0) return
			replaceInputTag(e.target, box.children[box.selection].innerText)
			e.preventDefault()
			break
		case "ArrowUp":
		case "ArrowDown":
		case "ArrowLeft":
		case "ArrowRight":
			removeSTBox()
			break
	}
}
