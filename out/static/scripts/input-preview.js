function renderInput(e)
{
	let pbox = document.getElementById("preview-box")
	// Remove existing previews
	for(let c = pbox.firstChild; c != null; c = pbox.firstChild)
		pbox.removeChild(c)

	const file = e.target.files[0]
	const type = file.type.split("/")[0]

	if(!(type == "image" || type == "video"))
		return

	let reader = new FileReader()
	reader.onload = function(ev) {
		let el
		switch(type)
		{
			case "image":
				el = document.createElement("img")
				el.src = ev.target.result
				break
			case "video":
				let src = document.createElement("source")
				src.src = ev.target.result
				el = document.createElement("video")
				el.appendChild(src)
				el.controls = true
				break
		}
		pbox.appendChild(el)
	}
	reader.readAsDataURL(file)
}
