package DataManager

import "html/template"

var archiveTemplates *template.Template

func init() {
	archiveTemplates = template.Must(template.New("post").Parse(templatePost))
	archiveTemplates = template.Must(archiveTemplates.New("list").Parse(templatePostList))
	archiveTemplates = template.Must(archiveTemplates.New("index").Parse(templateIndex))
}

const templatePost = `
<html>
<head>
</head>
<body>
	<ul>
	{{range .Tags}}
		<li>{{.}}</li>
	{{end}}
	</ul>
	<div>
	{{if eq .Post.Mime.Type "image"}}
		<img src="../../{{.FilePath}}" alt="/ipfs/{{.Post.Hash}}">
	{{else}}
		<a href="../../{{.FilePath}}">Download</a>
	{{end}}
		<div>{{.Post.Mime}}</div>
	</div>
	
</body>
</html>
`

const templatePostList = `
<html>
<head>
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
</head>
<body>
	<h1>The Permanent Booru</h1>
	<h3>Mini archive v{{.Version}}</h3>
	<p>Find the main site over at <br>
	<a href="http://owmvhpxyisu6fgd7r2fcswgavs7jly4znldaey33utadwmgbbp4pysad.onion/">owmvhpxyisu6fgd7r2fcswgavs7jly4znldaey33utadwmgbbp4pysad.onion>/a><br>
	<a href="http://kycklingar.i2p/">kycklingar.i2p</a>
	</p>
	<a href="./list/1"><div>Start Browsing</div></a>
</body>
</html>
`
