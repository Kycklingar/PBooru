package forum

import (
	"regexp"
	"html"
)

const (
	regReference = "(&gt;&gt;([0-9]+))(\\s|$)"
	regGreenText = "(?m)(?:^&gt;[^&gt;]|^&gt;&gt;(?:&gt;)+).*"
	regMention   = "(@([0-9]+))(\\s|$)"
	regNewLine   = "\\n"
)

var (
	reference *regexp.Regexp
	greenText *regexp.Regexp
	mention   *regexp.Regexp
	newLine   *regexp.Regexp
)

func init() {
	var err error

	reference, err = regexp.Compile(regReference)
	if err != nil {
		panic(err)
	}

	greenText, err = regexp.Compile(regGreenText)
	if err != nil {
		panic(err)
	}

	mention, err = regexp.Compile(regMention)
	if err != nil {
		panic(err)
	}

	newLine, err = regexp.Compile(regNewLine)
	if err != nil {
		panic(err)
	}
}

type Body string

func (b Body) Compile() string {
	escaped := html.EscapeString(string(b))

	out := reference.ReplaceAllString(escaped, "<a class=\"ref\" href=\"#$2\">$1</a>")
	out = greenText.ReplaceAllString(out, "<span class=\"greentext\">$0</span>")
	out = mention.ReplaceAllString(out, "<a class=\"mention\">$1</a>")
	out = newLine.ReplaceAllString(out, "<br>")

	return out
}

//func Mentions(text string) {
//
//	references := reference.FindAllStringSubmatch(text, -1)
//	mentions := mention.FindAllStringSubmatch(b)
//}
