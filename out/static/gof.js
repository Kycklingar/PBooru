
var canvas = document.getElementById("canvas")
//canvas.width = screen.width
//canvas.height = screen.height

canvas.width = canvas.parentElement.offsetWidth
canvas.height = canvas.parentElement.offsetHeight
var context = canvas.getContext("2d")

var boardSizeX = canvas.width / 28
var boardSizeY = canvas.height / 28

livingTiles = []

function initTiles()
{
	for(let i = 0; i < boardSizeX * boardSizeY / 10; i++)
	{
		let x = Math.floor((Math.random() * boardSizeX))
		let y = Math.floor((Math.random() * boardSizeY))
		setLife(livingTiles, x, y)
	}


	// Spaceship
	// setLife(livingTiles, 5, 5)
	// setLife(livingTiles, 6, 5)
	// setLife(livingTiles, 7, 5)
	// setLife(livingTiles, 7, 6)
	// setLife(livingTiles, 6, 7)

}

function updateTiles(tiles)
{
	let newLivingTiles = []

	let ttc = tilesToCheck(tiles)
	//console.log(ttc)
	for(let i = 0; i < ttc.length; i++)
	{
		let x = ttc[i].x
		let y = ttc[i].y
		let alive = closePop(x, y, tiles)
		//console.log(ttc[i], alive)
		if(!isAlive(x, y, tiles) && alive == 3)
		{
			//console.log("Rise from the dead:", x, y)
			setLife(newLivingTiles, x, y)
		}
		else if(alive >= 2 && alive <= 3 && isAlive(x, y, tiles))
		{
			//console.log("Survivor:", x, y)
			setLife(newLivingTiles, x, y)
		}

	}

	return newLivingTiles
}

function tilesToCheck(tiles)
{
	let allTiles = []
	for(let i = 0; i < tiles.length; i++)
	{
		for(let x = -1; x <= 1; x++)
		{
			for(let y = -1; y <=1; y++)
			{
				//console.log("Check this", tiles[i])
				if(
					tiles[i].x + x < 0 
					||	tiles[i].x + x > boardSizeX
					||	tiles[i].y + y < 0
					||	tiles[i].y + y > boardSizeY
				)
				{
					//console.log("Offscreen lifeform")
					continue
				}
				setLife(allTiles, tiles[i].x + x, tiles[i].y + y)
			}
		}
	}

	return allTiles
}

function setLife(tiles, x, y)
{
	for(let i = 0; i < tiles.length; i++)
	{
		if(tiles[i].x == x && tiles[i].y == y)
		{
			return
		}
	}
	tiles.push(newTile(x, y))
}

function newTile(x, y)
{
	return {
		"x":x,
		"y":y
	}
}

function isAlive(x, y, tiles)
{
	for(let i = 0; i < tiles.length; i++)
	{
		if(tiles[i].x == x && tiles[i].y == y)
		{
			return true
		}
	}
	return false
}

function closePop(x, y, tiles)
{
	let alive = 0
	for(let i = 0; i < tiles.length; i++)
	{
		if(
			!(
				tiles[i].x == x
				&& tiles[i].y == y
			)
			&& (
				tiles[i].x >= x-1
				&& tiles[i].x <= x+1
			)
			&& (
				tiles[i].y >= y-1
				&& tiles[i].y <= y+1
			)
		)
		{
			alive++
		}
	}

	return alive
}

function drawBoard(ctx, tiles)
{
	ctx.beginPath()
	ctx.fillStyle = "rgba(0, 0, 0, 0)"
	ctx.clearRect(0, 0, canvas.width, canvas.height)

	let sw = canvas.width / boardSizeX
	let sh = canvas.height / boardSizeY

	const hbw = boardSizeX / 2
	const hbh = boardSizeY / 2

	// dropshadow
	for(let i = 0; i < tiles.length; i++)
	{
		const [x, y] = [tiles[i].x, tiles[i].y]
		const ofx = ((x - hbw) / hbw) * sw / 3
		const ofy = ((y - hbh) / hbh) * sh / 3

		const r = x / boardSizeX * 222
		const b = y / boardSizeY * 222

		ctx.beginPath()
		ctx.fillStyle = `rgb(${r}, 100, ${b})`
		ctx.rect(x * sw + ofx - 1, y * sh + ofy - 1, sw + 2, sh + 2)
		ctx.fill()
	}

	// cell
	for(let i = 0; i < tiles.length; i++)
	{
		const [x, y] = [tiles[i].x*sw, tiles[i].y*sh]

		ctx.beginPath()
		ctx.fillStyle = "black"
		ctx.rect(x, y, sw, sh)
		ctx.fill()
	}
}

initTiles()
drawBoard(context, livingTiles)

function draw()
{
	livingTiles = updateTiles(livingTiles)
	drawBoard(context, livingTiles)
}


setInterval(draw, 200)
