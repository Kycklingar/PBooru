package DataManager

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/frustra/bbcode"
)

func createCompiler(q querier, gateway string) bbcode.Compiler {
	cmp := bbcode.NewCompiler(true, true)
	cmp.SetTag("img", nil)
	cmp.SetTag("post", func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
		id, err := strconv.Atoi(node.GetOpeningTag().Value)
		if err != nil {
			return nil, false
		}

		post := NewPost()
		if err = post.SetID(q, id); err != nil {
			return nil, false
		}
		a := bbcode.NewHTMLTag("")
		a.Name = "a"
		a.Attrs["href"] = fmt.Sprintf("/post/%d/%s", post.QID(q), post.QHash(q))
		post.QThumbnails(q)
		img := bbcode.NewHTMLTag("")
		img.Name = "img"
		img.Attrs["src"] = gateway + "/ipfs/" + post.ClosestThumbnail(250)
		img.Attrs["style"] = "max-width:250px; max-height:250px;"

		a.AppendChild(img)

		return a, true
	})
	cmp.SetTag("ref", func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
		ref := node.GetOpeningTag().Value

		a := bbcode.NewHTMLTag("")
		a.Name = "a"
		a.Attrs["href"] = ref
		return a, true
	})
	cmp.SetTag("greentext", func(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
		val := node.GetOpeningTag().Value
		a := bbcode.NewHTMLTag("")
		a.Name = "span"
		a.Attrs["class"] = "greentext"
		a.Attrs["sl"] = val
		return a, true
	})

	cmp.SetTag("url", urlNode)

	return cmp
}

func compileBBCode(q querier, text, gateway string) string {
	cmp := createCompiler(q, gateway)

	// Comment reference
	reg, err := regexp.Compile("#([0-9]+)\\b(\\s|$)")
	if err != nil {
		log.Println(err)
		return text
	}

	// Greentext
	gt, err := regexp.Compile("(?m)^>.*")
	if err != nil {
		log.Println(err)
		return text
	}

	out := reg.ReplaceAllString(text, "[ref=#c$1]#$1[/ref] ")
	out = gt.ReplaceAllString(out, "[greentext]$0[/greentext]")
	return cmp.Compile(out)
}

func urlNode(node *bbcode.BBCodeNode) (*bbcode.HTMLTag, bool) {
	out := bbcode.NewHTMLTag("")
	out.Name = "a"
	value := node.GetOpeningTag().Value
	if value == "" {
		text := bbcode.CompileText(node)
		if len(text) > 0 {
			out.Attrs["href"] = validURL(text)
		}
	} else {
		out.Attrs["href"] = validURL(value)
	}

	return out, true
}

func validURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}

	if u.Scheme == "javascript" {
		return ""
	}

	return u.String()
}