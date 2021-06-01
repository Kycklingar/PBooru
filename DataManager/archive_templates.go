package DataManager

import "html/template"

var archiveTemplates *template.Template

func init() {
	archiveTemplates = template.Must(template.New("post").Parse(templatePost))
	archiveTemplates = template.Must(archiveTemplates.New("list").Parse(templatePostList))
}

const templatePost = `
<html>
<head>
</head>
<body>
	<ul>
	{{range .Tags}}
		{{.}}
	{{end}}
	</ul>
	<div>
	{{if eq .Post.Mime.Type "image"}}
		<img src="../../{{.FilePath}}" alt="/ipfs/{{.Post.Hash}}">
	{{else}}
		<a href="../../{{.FilePath}}">Download</a>
	{{end}}
		<span>{{.Post.Mime}}</span>
	</div>
	
</body>
</html>
`

const templatePostList = `
<html>
<head>
</head>
<body>
	<div>
	{{range .}}
		<a href="../{{.PostPath}}">
		{{- with .ThumbnailPath -}}
			<img src="../{{.}}">
		{{- else -}}
			<div>{{.Post.Mime}}</div>
		{{- end -}}
		</a>
	{{end}}
	</div>
</body>
</html>
`
