package DataManager

import (
	"bytes"
	"html"

	dmp "github.com/sergi/go-diff/diffmatchpatch"
)

func diffHtml(a, b string) string {
	differ := dmp.New()
	return htmlDiff(differ.DiffMain(a, b, false))
}

func htmlDiff(diffs []dmp.Diff) string {
	var buff bytes.Buffer
	for _, diff := range diffs {
		//text := strings.Replace(html.EscapeString(diff.Text), "\n", "<br>", -1)
		text := html.EscapeString(diff.Text)
		switch diff.Type {
		case dmp.DiffInsert:
			buff.WriteString("<span class=\"new\">+")
			buff.WriteString(text)
			buff.WriteString("</span>")
		case dmp.DiffDelete:
			buff.WriteString("<span class=\"del\">-")
			buff.WriteString(text)
			buff.WriteString("</span>")
		case dmp.DiffEqual:
			buff.WriteString("<span>")
			buff.WriteString(text)
			buff.WriteString("</span>")
		}
	}

	return buff.String()
}
