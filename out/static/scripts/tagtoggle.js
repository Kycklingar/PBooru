let currentTagInputElement

function setInputElement(element)
{
	currentTagInputElement = element
}
function setTagInput(ev)
{
	setInputElement(ev.target)
}

function appendQuery(ev)
{
	if(!currentTagInputElement)
		return

	let caller = ev.target

	let removed = toggleInputTag(currentTagInputElement, caller.dataset.tag)

	switch(removed) {
		case true: caller.innerText = "+"; break
		case false: caller.innerText = "-"; break
	}
}

document.addEventListener("DOMContentLoaded", function(){
	document.querySelectorAll(".tag-toggle").forEach((button)=>{
		button.addEventListener("click", appendQuery)
		button.style.cursor = "pointer"
	})
	document.querySelectorAll(".tag-input").forEach((inputElement)=>{
		if(!currentTagInputElement)
			setInputElement(inputElement)

		inputElement.addEventListener("focus", setTagInput)
	})
})
