package forum

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"strconv"
)

const (
	regReference = "(&gt;&gt;([0-9]+))([^a-zA-Z]|$)"
	regGreenText = "(?m)(?:^&gt;[^&gt;]|^&gt;&gt;(?:&gt;)+).*"
	regMention   = "(@([0-9]+))([^a-zA-Z]|$)"
	regNewLine   = "\\n"
	regCodeBlock = "\\[\\[\\[\\n(.+)\\n\\]\\]\\]"
)

var (
	reference *regexp.Regexp
	greenText *regexp.Regexp
	mention   *regexp.Regexp
	newLine   *regexp.Regexp
	codeBlock *regexp.Regexp
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

	//codeBlock, err = regex.Compile(regCodeBlock)
}

type Body struct {
	Text       string
	Backlinks  []Post
	References map[int]Post
}

func (b Body) Compile() string {
	escaped := html.EscapeString(b.Text)

	//out := reference.ReplaceAllString(escaped, "<a class=\"ref\" href=\"#$2\">$1</a>$3")
	out := reference.ReplaceAllStringFunc(escaped, b.refHtml)
	out = greenText.ReplaceAllString(out, "<span class=\"greentext\">$0</span>")
	out = mention.ReplaceAllString(out, "<a class=\"mention\">$1</a>$3")
	out = newLine.ReplaceAllString(out, "<br>")

	return out
}

func (b Body) Mentions() []int {
	var r []int
	bod := html.EscapeString(b.Text)
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

func (b Body) refHtml(in string) string {
	ref := reference.FindAllStringSubmatch(in, -1)
	id, _ := strconv.Atoi(ref[0][2])
	rep, ok := b.References[id]

	var a string
	if ok {
		a = fmt.Sprintf(`<span class="ref"><a href="#%s">%s</a><span class="thread-post"><span class="title">%s</span><br><span>%s</span></span></span>%s`,
			ref[0][2],
			ref[0][1],
			rep.Title,
			rep.Body.Compile(),
			ref[0][3],
		)
	} else {
		a = fmt.Sprintf(`<span class="ref"><span class="dead">%s</span></span>`,
			ref[0][1],
		)
	}

	//a := fmt.Sprintf("<span class=\"ref\"><a href=\"#%s\">%s</a>%s%s</span>", ref[0][2], ref[0][1], inlinePost, ref[0][3])
	fmt.Println(a)
	return a
}
