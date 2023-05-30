let MenuContexts = {}

function MenuItem(text, handler)
{
	const item = document.createElement("div")
	item.innerText = text
	item.className = "item"
	item.addEventListener("click", (e) => {if(!handler(e)) e.stopPropagation()})
	return item
}

function MenuGroup(text, menuItems)
{
	const group = document.createElement("div")
	group.className = "group"

	const header = document.createElement("div")
	header.innerText = text
	header.className = "header"
	group.appendChild(header)
	menuItems.forEach(item => group.appendChild(item))
	return group
}

function newContextMenu(e)
{
	freeContextMenu()
	const target = e.currentTarget
	const menuItemsConstructor = MenuContexts[target.dataset.contextMenu]
	if(!menuItemsConstructor) return

	e.preventDefault()
	e.stopPropagation()

	const menuItems = menuItemsConstructor(target)
	const menu = document.createElement("div")

	menu.id = "context-menu"
	menu.style.position = "fixed"
	menu.style.zIndex = "9999"

	menuItems.forEach(item => menu.appendChild(item))

	document.body.appendChild(menu)

	const y = Math.max(0, menu.offsetHeight + e.clientY - window.innerHeight)
	const x = Math.max(0, menu.offsetWidth + e.clientX - window.innerWidth)

	menu.style.top = e.clientY - y
	menu.style.left = e.clientX - x

	window.addEventListener("click", freeContextMenu)
	window.addEventListener("contextmenu", freeContextMenu)
}

function freeContextMenu()
{
	window.removeEventListener("click", freeContextMenu)
	const contextMenu = document.getElementById("context-menu")
	contextMenu?.parentElement.removeChild(contextMenu)
}
