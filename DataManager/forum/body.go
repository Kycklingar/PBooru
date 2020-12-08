package forum

import (
	"regexp"
	"html"
	"log"
	"strconv"
)

const (
	regReference = "(&gt;&gt;([0-9]+))([^a-zA-Z]|$)"
	regGreenText = "(?m)(?:^&gt;[^&gt;]|^&gt;&gt;(?:&gt;)+).*"
	regMention   = "(@([0-9]+))([^a-zA-Z]|$)"
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

	out := reference.ReplaceAllString(escaped, "<a class=\"ref\" href=\"#$2\">$1</a>$3")
	out = greenText.ReplaceAllString(out, "<span class=\"greentext\">$0</span>")
	out = mention.ReplaceAllString(out, "<a class=\"mention\">$1</a>$3")
	out = newLine.ReplaceAllString(out, "<br>")

	return out
}

func (b Body) Mentions() []int {
	var r []int
	bod := html.EscapeString(string(b))
	refs := reference.FindAllStringSubmatch(bod, -1)
	for i := range refs {
		rid, err := strconv.Atoi(refs[i][2])
		if err != nil {
			log.Println(err)
			continue
		}
		r = append(r, rid)
	}
	//mentions := mention.FindAllStringSubmatch(b)

	return r
}
