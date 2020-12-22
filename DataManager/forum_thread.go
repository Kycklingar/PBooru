package DataManager

type ForumThread struct {
	ReplyCount int
	Head ForumPost
	Bumped ts.Timestamp

	Posts []*ForumPost

	allPosts map[int]*ForumPost
}

func (thread ForumThread) CompileBodies() {
	var compiledPosts map[int]string

	for k, p := range thread.allPosts {
		compiledPosts[k] = p.Compile()
	}

	for i := range thread.Posts {
		thread.Posts[i].InsertRefs(compiledPosts)
	}

}
