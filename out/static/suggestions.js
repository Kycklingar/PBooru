
document.onclick = removeSTBox

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

function replaceQuery(input, tag) {
	var a = input.value.lastIndexOf(",")
	var b = input.value.lastIndexOf("\n")
	var lastIndex
	if (a > b)
		lastIndex = a
	else
		lastIndex = b


	var append = ", "

	if (input.value[lastIndex] == "\n")
    		append = input.value[lastIndex]
		
    input.value = input.value.slice(0, lastIndex + 1) + tag + append
}

function suggestionBox(caller) {
    var str = caller.value.slice(caller.value.replace(/\n/g, ",").lastIndexOf(",") + 1).trim()
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
                sp.style.color = "black"
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
                }
                stb.appendChild(sp)
            }
        }
    }
    req.open("GET", "/api/v1/suggesttags?tags=" + str, true)
    req.send()
}

function setSTBoxTimeout(caller) {
    clearTimeout(timeout)
    timeout = setTimeout(suggestionBox, 400, caller)
}

var timeout

{
    let sti = document.getElementsByClassName("stinput")
    for(let i = 0; i < sti.length; i++){
        sti[i].oninput = function () { setSTBoxTimeout(this) }
        sti[i].autocomplete = "off"
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
    }
    // Up
    else if(e.keyCode == 38){
        box.selection -= 1
    }

    updateSelection(box)
}
