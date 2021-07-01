package DataManager

import "html/template"

var archiveTemplates *template.Template

func init() {
	archiveTemplates = template.Must(template.New("post").Parse(templatePost))
	archiveTemplates = template.Must(archiveTemplates.New("list").Parse(templatePostList))
	archiveTemplates = template.Must(archiveTemplates.New("index").Parse(templateIndex))
}

const templateCSS = `
.content {
	display: flex;
}

.content img, .content video {
	max-width: 100%;
}

.tag-list {
	list-style: none;
	max-width: 20em;
	white-space: nowrap;
	padding-left: 1em;
	padding-right: 1em;
}
`

const templatePost = `
<html>
<head>
	<link rel="stylesheet" href="../../style.css">
</head>
<body>
	<div class="content">
		<ul class="tag-list">
		{{range .Tags}}
			<li>{{.}}</li>
		{{end}}
		</ul>
		<div>
		{{if eq .Post.Mime.Type "image"}}
			<div><img src="../../{{.FilePath}}" alt="/ipfs/{{.Post.Hash}}"></div>
		{{else if eq .Post.Mime.Type "video"}}
			<video src="../../{{.FilePath}}" controls loop>
					/ipfs/{{.Post.Hash}}
			</video>
		{{end}}
			<a href="../../{{.FilePath}}">Download</a>
			<div>{{.Post.Mime}}</div>
		</div>
	</div>
</body>
</html>
`

const templatePostList = `
<html>
<head>
	<link rel="stylesheet" href="../style.css">
</head>
<body>
	<div class="posts">
	{{range .Posts}}
		<a href="../{{.PostPath}}">
		{{- with .ThumbnailPath -}}
			<img src="../{{.}}">
		{{- else -}}
			<span>{{.Post.Mime}}</span>
		{{- end -}}
		</a>
	{{end}}
	</div>
	<div class="paginator">
	{{range .Pag.Paginate}}
		{{if .Current}}
		<span style="font-size: 2em;">{{.Val}}</span>
		{{else}}
		<span><a href="{{.Href}}">{{.Val}}</a></span>
		{{end}}
	{{end}}
	</div>
</body>
</html>
`

const templateIndex = `
<html>
<head>
	<link rel="stylesheet" href="./style.css">
</head>
<body>
	<h1>The Permanent Booru</h1>
	<h3>Mini archive v{{.Version}}</h3>
	<p>Find the main site over at <br>
	<a href="http://owmvhpxyisu6fgd7r2fcswgavs7jly4znldaey33utadwmgbbp4pysad.onion/">owmvhpxyisu6fgd7r2fcswgavs7jly4znldaey33utadwmgbbp4pysad.onion</a><br>
	<a href="http://kycklingar.i2p/">kycklingar.i2p</a>
	</p>
	<ul>
		{{with .Ident.And}}
		<li>And
			<ul>
			{{range .}}
				<li>{{.}}</li>
			{{end}}
			</ul>
		</li>
		{{end}}

		{{with .Ident.Or}}
		<li>Or
			<ul>
			{{range .}}
				<li>{{.}}</li>
			{{end}}
			</ul>
		</li>
		{{end}}

		{{with .Ident.Filter}}
		<li>Filter
			<ul>
			{{range .}}
				<li>{{.}}</li>
			{{end}}
			</ul>
		</li>
		{{end}}

		{{with .Ident.Unless}}
		<li>Unless
			<ul>
			{{range .}}
				<li>{{.}}</li>
			{{end}}
			</ul>
		</li>
		{{end}}
		{{with .Ident.Mimes}}
		<li>Mimes
			<ul>
			{{range .}}
				<li>{{.}}</li>
			{{end}}
			</ul>
		</li>
		{{end}}

	</ul>
	<a href="./list/1"><div>Start Browsing</div></a>
</body>
</html>
`
