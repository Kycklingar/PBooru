MenuContexts.thumbnail = (target) => [
	MenuGroup("Copy", [
		CopyItem("id", target),
		CopyItem("cid", target),
		CopyItem("sha256", target),
		CopyItem("md5", target),
	]),
	MenuGroup("Find Similar", [
		SimilarItem("Strong", target, 1),
		SimilarItem("Medium", target, 5),
		SimilarItem("Weak", target, 10),
	]),
]

document.querySelectorAll(".thumbnail")
	.forEach(t => t.addEventListener("contextmenu", newContextMenu))

const SimilarItem = (text, target, distance) => MenuItem(
	text,
	(e) => window.location = `/similar?id=${target.dataset.id}&distance=${distance}`
)

const CopyItem = (property, target) => MenuItem(
	`${capitalize(property)}\t[${truncate(target.dataset[property])}]`,
	(e) => navigator.clipboard.writeText(target.dataset[property])
)

const capitalize = (s) => s[0].toUpperCase() + s.slice(1)
const truncate = (s) => {
	if(s.length > 10) return `${s.substring(0,5)}..${s.substring(s.length-5)}`
	else return s
}
