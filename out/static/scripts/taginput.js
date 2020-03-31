var currentTagInput = null

function setInput(el)
{
	currentTagInput = el
}

function appendQuery(caller, tag)
{
    if(currentTagInput == null)
    {
    	return
    }

    let inp = currentTagInput.value.split(",")
    let ret = []
    let n = true

    for(let i = 0; i < inp.length; i++)
    {
	if(inp[i].trim() != tag)
	{
	    if(inp[i].trim().length >= 1)
	    {
		ret.push(inp[i].trim())
	    }
	}
	else
	{
	    n = false
	}
    }
    if(n)
    {
	ret.push(tag)
	caller.innerHTML = "-"
    }
    else
    {
	caller.innerHTML = "+"
    }
    currentTagInput.value = ""
    for(let i = 0; i < ret.length; i++)
    {
	if(i == ret.length - 1)
	{
	    currentTagInput.value += ret[i]
	}
	else
	{
	    currentTagInput.value += ret[i] + ", "
	}
    }
}
