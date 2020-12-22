package DataManager

type ForumPost struct {
	Thread int
	Id int
	Title string
	Body string
	Poster *User
	Created ts.Timestamp

	compiledPost string
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
	reference, err = regexp.MustCompile(regReference)
	greenText, err = regexp.MustCompile(regGreenText)
	mention, err = regexp.MustCompile(regMention)
	newLine, err = regexp.MustCompile(regNewLine)
}

func (p ForumPost) Compile() string {
	p.compiledPost = fmt.Sprintf(`
		<div id="%d">
			<span>%s</span>
			<span>%s</span>
			<h3>%s</h3
			<div>%s</div>
		</div>
		`,
		p.Id,
		p.Title,
		p.CompileBody(),
	)

	return p.compiledPost
}

func (p ForumPost) CompileBody() string {
	escaped := html.EscapeString(b.Text)

	//out := reference.ReplaceAllString(escaped, "<a class=\"ref\" href=\"#$2\">$1</a>$3")
	//out := reference.ReplaceAllStringFunc(escaped, b.refHtml)
	out = greenText.ReplaceAllString(out, "<span class=\"greentext\">$0</span>")
	out = mention.ReplaceAllString(out, "<a class=\"mention\">$1</a>$3")
	out = newLine.ReplaceAllString(out, "<br>")

	return out
}

func (p *ForumPost) InsertRefs(compiledPosts map[int]string) {
	repl := func(s string) string {
		r := reference.FindAllStringSubmatch(s, 1)
		id, err := strconv.Aoti(r[0][1])
		if cmp, ok := compiledPosts[id]; ok {
			return fmt.Sprintf(`
				<span class="ref">
					%s
					<div class="thread-post">
						%s
					</div>
				</span>
				`,
				s,
				compiledPosts[id])
		}

		return fmt.Sprintf(`
			<span class="ref dead">%s</span>
			`,
			s,
		)
	}

	p.compiledPost = reference.ReplaceAllStringFunc(p.compiledPost, repl)
}
