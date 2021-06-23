
//document.onclick = removeSTBox

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

function firstIndex(start, delim, val)
{
	let state = 0

	for (let i = start - 1; i >= 0; i--)
	{
		switch (state)
		{
			case 0:
				if (val[i] == delim)
				{
					state = 1
				}
				break
			case 1:
				if (val[i] == "\\")
				{
					// Escaped character, continue search
					state = 0
					continue
				}
				// Start found
				return i+2
				break
		}
	}

	// Start from beginning
	return 0
}

function lastIndex(start, delim, val)
{
	let state = 0

	for (let i = start; i < val.length; i++)
	{
		switch (state)
		{
			case 0:
				if (val[i] == "\\")
				{
					state = 1
					continue
				}
				if (val[i] == delim)
					return i-1
				break
			case 1:
				state = 0
				break
		}
	}

	return val.length
}

function escapeTag(tag)
{
	return tag.replaceAll(",", "\\,")
}

function replaceQuery(input, tag) {
	let delim = ","
	let padding = " "
	if (input.tagName == "TEXTAREA")
	{
		delim = "\n"
		padding = ""
	} else {
		tag = escapeTag(tag)
	}

	let first = firstIndex(input.selectionStart, delim, input.value)
	let last = lastIndex(input.selectionStart, delim, input.value)

	let beg = input.value.slice(0, first)
	let end = input.value.slice(last+1, input.value.length)
	input.value = beg + padding + tag + delim + padding + end
	input.selectionStart = input.selectionEnd = first + tag.length + (padding.length * 2)
}

function suggestionBox(caller) {
	let delim = caller.tagName == "TEXTAREA" ? "\n" : ","

	let first = firstIndex(caller.selectionStart, delim, caller.value)
	let last = lastIndex(caller.selectionStart, delim, caller.value)

	let str = caller.value.slice(first, last+1)

	if (str.length <= 2) {
		removeSTBox()
		return
	}

	caller.addEventListener("keydown", keyDown)

	var req = new XMLHttpRequest()
	req.onreadystatechange = function () {
		if (this.readyState == 4 && this.status == 200) {
			var tags = JSON.parse(this.responseText)
			removeSTBox()

			if (tags == null) {
				return
			}

			var stb = document.createElement("div")
			stb.id = "stbox"
			caller.parentElement.insertBefore(stb, caller.nextSibling)

			for (var i = 0; i < tags.length && i < 5; i++) {
				let sp = document.createElement("div")
				sp.style.fontSize = "16px"
				sp.style.cursor = "pointer"
				sp.className = "tag namespace-" + tags[i].Namespace
				if (tags[i].Namespace == "none") {
					sp.innerText = tags[i].Tag
				}
				else {
					sp.innerText = tags[i].Namespace + ":" + tags[i].Tag
				}
				sp.onclick = function () {
					replaceQuery(caller, sp.innerText)
					while (stb.children.length > 0) {
						stb.removeChild(stb.children[0])
					}
					caller.focus()
					removeSTBox()
				}
				stb.appendChild(sp)
			}
		}
	}

	let sp = new URLSearchParams()
	sp.append("delim", delim)
	sp.append("tags", str)
	req.open("GET", "/api/v1/suggesttags?" + sp.toString(), true)
	req.send()
}

function setSTBoxTimeout(caller) {
	clearTimeout(timeout)
	timeout = setTimeout(suggestionBox, 200, caller)
}

var timeout

{
	let sti = document.getElementsByClassName("stinput")
	for(let i = 0; i < sti.length; i++){
		sti[i].oninput = function () { setSTBoxTimeout(this) }
		sti[i].autocomplete = "off"
		//sti[i].addEventListener("focusout", removeSTBox)
	}
}


function updateSelection(box)
{
	if(box.selection > box.children.length - 1)
	{
		box.selection = -1
	}
	else if(box.selection < -1)
	{
		box.selection = box.children.length - 1
	}
	for(let i = 0; i < box.children.length; i++)
	{
		box.children[i].style.border = "none"
	}
	if(box.selection == -1)
	{
		return
	}
	box.children[box.selection].style.border = "dotted"
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

	// Esc, tab
	if(e.keyCode == 27 || e.keyCode == 9)
	{
		removeSTBox()
		return
	}

	// Right
	if(e.keyCode == 39)
	{
		if(box.selection < 0)
		{
			return
		}
		replaceQuery(e.target, box.children[box.selection].innerText)
		removeSTBox()
		return
	}

	// Down
	if(e.keyCode == 40){
		box.selection += 1
		e.preventDefault()
	}
	// Up
	else if(e.keyCode == 38){
		box.selection -= 1
		e.preventDefault()
	}

	updateSelection(box)
}
