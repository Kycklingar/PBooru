package DataManager

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"strconv"

	ts "github.com/kycklingar/PBooru/DataManager/timestamp"
)

type ForumPost struct {
	Thread  int
	Id      int
	Title   string
	Body    string
	Poster  *User
	Created ts.Timestamp

	CompiledBody string
	CompiledWithRefs string
}

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
	reference = regexp.MustCompile(regReference)
	greenText = regexp.MustCompile(regGreenText)
	mention = regexp.MustCompile(regMention)
	newLine = regexp.MustCompile(regNewLine)
}

func (p ForumPost) Compile() string {
		name := "Anonymous"
	if p.Poster != nil {
		name = p.Poster.Name
	}

	p.CompiledBody = fmt.Sprintf(`
		<span class="thread-username">%s</span>
		<span class="thread-id">#%d</span>
		<span class="thread-timestamp">%s</span>
		<h3 class="thread-title">%s</h3>
		<span class="thread-body">%s</span>
		`,
		name,
		p.Id,
		p.Created.Elapsed(),
		p.Title,
		p.CompiledBody,
	)

	return p.CompiledBody
}

func (p *ForumPost) CompileBody() string {
	escaped := html.EscapeString(p.Body)

	//out := reference.ReplaceAllString(escaped, "<a class=\"ref\" href=\"#$2\">$1</a>$3")
	//out := reference.ReplaceAllStringFunc(escaped, b.refHtml)
	out := greenText.ReplaceAllString(escaped, "<span class=\"greentext\">$0</span>")
	out = mention.ReplaceAllString(out, "<a class=\"mention\">$1</a>$3")
	out = newLine.ReplaceAllString(out, "<br>")

	p.CompiledBody = out

	return out
}

func (p *ForumPost) InsertRefs(compiledPosts map[int]*ForumPost) {
	repl := func(s string) string {
		r := reference.FindAllStringSubmatch(s, 1)
		id, err := strconv.Atoi(r[0][2])
		if err != nil {
			log.Println(err)
		}

		fmt.Println(id)
		fmt.Println(compiledPosts)
		if cmp, ok := compiledPosts[id]; ok {
			fmt.Println(cmp, ok)
			return fmt.Sprintf(`
				<a href="../%d/#%d" class="ref">
					%s
					<div class="thread-post">
						%s
					</div>
				</a>
				`,
				cmp.Thread,
				id,
				s,
				cmp.Compile(),
			)
		}

		return fmt.Sprintf(`
			<span class="ref dead">%s</span>
			`,
			s,
		)
	}

	p.CompiledWithRefs = reference.ReplaceAllStringFunc(p.CompiledBody, repl)
}
