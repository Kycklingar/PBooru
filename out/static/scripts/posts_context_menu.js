MenuContexts.thumbnail = (target) => [
	MenuGroup("Find Similar", [
		MenuItem("Strong", (e) => window.location = `/similar?id=${target.dataset.postId}&distance=1`),
		MenuItem("Medium", (e) => window.location = `/similar?id=${target.dataset.postId}&distance=5`),
		MenuItem("Weak", (e) => window.location = `/similar?id=${target.dataset.postId}&distance=10`),
	]),
]

document.querySelectorAll(".thumbnail")
	.forEach(t => t.addEventListener("contextmenu", newContextMenu))
