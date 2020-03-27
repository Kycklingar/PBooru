function qaRegister(ctitle, title, id)
{
	let chapters = qaChapters()

	for (let i = 0; i < chapters.length; i++)
	{
		if (chapters[i].ID == id)
		{
			return
		}
	}

	chapters.unshift(
		{
			"comicTitle": ctitle,
			"title": title,
			"ID": id,
			"lastPage": 0,
		}
	)

	qaSet(chapters)
}

function qaRemove(chID)
{
	let chapters = qaChapters()

	for (let i = 0; i < chapters.length; i++)
	{
		if (chapters[i].ID == chID)
		{
			chapters.splice(i, 1)
			qaSet(chapters)
		}
	}
}

function qaQuery(chID)
{
	let chapters = qaChapters()
	
	for (let i = 0; i < chapters.length; i++)
	{
		if (chapters[i].ID == chID)
			return chapters[i]
	}

	return null
}

function qaChapters()
{
	let chapters = JSON.parse(window.localStorage.getItem("chapters"))
	if (chapters == null)
		chapters = []
	return chapters
}

function qaSet(chapters)
{
	window.localStorage.setItem("chapters", JSON.stringify(chapters))
}

function qaIncrement(chID, currentPage)
{
	let chapter = qaQuery(chID)
	if (chapter == null)
		return

	qaRemove(chapter.ID)

	chapter.lastPage = Number(currentPage) + 1

	let chapters = qaChapters()
	chapters.unshift(chapter)
	qaSet(chapters)
}
