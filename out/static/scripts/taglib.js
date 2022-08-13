class TagParser
{
	constructor(input)
	{
		this.input = input
		this.state = this.stateNext
	}

	index = 0;
	tag = "";

	*yieldTag() {
		this.tag = this.tag.trim()
		yield this.tag
		this.tag = ""
	}

	getc() {
		if(this.index < this.input.length)
			return this.input[this.index++]
		return null
	}

	*stateNext() {
		let c
		while((c = this.getc()) != null) {
			switch(c) {
				case "\n":
				case ",":
					yield *this.yieldTag()
					break
				case "\\":
					this.tag += c
					this.state = this.stateEscape
					return
				default:
					this.tag += c
			}
		}

		yield *this.yieldTag()

		this.state = null
	}

	*stateEscape() {
		let c = this.getc()
		if(!c) return

		switch(c) {
			case "\n": break
			default: this.tag += c
		}

		this.state = this.stateNext
	}

	*parse() {
		if(!this.input.length)
			return
		while(this.state != null)
			yield *this.state()
	}
}

// replace the tag at the cursor
function replaceInputTag(inputElement, newTag)
{
	let parser = new TagParser(inputElement.value)
	let tags = []
	let tagIndex = 0

	let append = function(tag) {
		tags.push(tag)
	}

	let check = function(tag) {
		if(parser.index > inputElement.selectionEnd) {
			append(newTag)
			f = append
			return
		}

		tagIndex++
		append(tag)
	}

	let f = check
	for(const tag of parser.parse())
		f(tag)

	// cursor at end
	if(f == check) {
		tags.pop()
		append(newTag)
	}

	writeToInput(inputElement, tags, tagIndex)
}

function toggleInputTag(inputElement, newTag)
{
	let parser = new TagParser(inputElement.value)
	let tags = []
	let removed = false

	for(const tag of parser.parse()) {
		if(tag == newTag)
			removed = true
		else tags.push(tag)
	}

	if(!removed)
		tags.push(newTag)

	writeToInput(inputElement, tags)

	return removed
}

function getTagAtCursor(inputElement)
{
	let tag, parser = new TagParser(inputElement.value)

	for(tag of parser.parse()) {
		if(parser.index > inputElement.selectionEnd)
			return tag
	}

	return tag
}

// write tags to input and place cursor at the end of tags[index]
function writeToInput(e, tags, tagIndex)
{
	if(!tags.length) {
		e.value = ""
		return
	}

	let sep
	switch(e.tagName) {
		case "TEXTAREA": sep = "\n"; break
		case "INPUT": sep = ", "; break
	}

	let cursorPosition = 0
	let res = tags[0]
	for(let i = 1; i < tags.length; i++) {
		res += sep + tags[i]
		if(i == tagIndex) cursorPosition = res.length
	}

	if(!cursorPosition)
		cursorPosition = res.length

	e.value = res
	e.selectionEnd = cursorPosition
}
