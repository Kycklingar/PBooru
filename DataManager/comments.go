package DataManager

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/frustra/bbcode"
)

// CommentModel is used to retriev and save comments
type CommentCollector struct {
	Comments []*Comment
}

type Comment struct {
	ID   int
	User *User
	Text string
	Time string
}

// Initialize a new comment
func NewComment() *Comment {
	return &Comment{User: NewUser()}
}

// Get the latest comments
func (cm *CommentCollector) Get(q querier, count int, daemon string) error {
	rows, err := q.Query("SELECT id, user_id, text, timestamp FROM comment_wall ORDER BY id DESC LIMIT ?", count)
	if err != nil {
		return err
	}

	for rows.Next() {
		c := NewComment()
		err = rows.Scan(&c.ID, &c.User.ID, &c.Text, &c.Time)
		if err != nil {
			rows.Close()
			return err
		}
		cm.Comments = append(cm.Comments, c)
	}
	rows.Close()

	for i := range cm.Comments {
		cm.Comments[i].Text = compileBBCode(q, cm.Comments[i].Text, daemon)
	}

	return rows.Err()
}

func compileBBCode(q querier, text, daemon string) string {
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

		img := bbcode.NewHTMLTag("")
		img.Name = "img"
		img.Attrs["src"] = daemon + "/ipfs/" + post.QThumb(q)
		img.Attrs["style"] = "max-width:250px; max-height:250px;"

		a.AppendChild(img)

		return a, true
	})
	return cmp.Compile(text)
}

//Save a new comment to the wall
func (cm *Comment) Save(userID int, text string) error {

	str := compileBBCode(DB, strings.TrimSpace(text), "")
	if len(str) < 3 {
		return fmt.Errorf("Post does not exist")
	}

	_, err := DB.Exec("INSERT INTO comment_wall(user_id, text) VALUES(?, ?)", userID, strings.TrimSpace(text))
	return err
}

func newPostComment() *PostComment {
	return &PostComment{User: NewUser(), Post: NewPost()}
}

type PostComment struct {
	ID   int
	Post *Post
	User *User
	Text string
	Time string
}

// Save a new comment on a post
func (pc *PostComment) Save(q querier) error {
	if pc.Text == "" || pc.Post.QID(q) == 0 || pc.User.QID(q) == 0 {
		return fmt.Errorf("expected: Text, PostID, UserID. Got: %s, %d, %d", pc.Text, pc.Post.ID, pc.User.ID)
	}

	_, err := DB.Exec("INSERT INTO post_comments(post_id, user_id, text) VALUES(?, ?, ?)", pc.Post.ID, pc.User.ID, pc.Text)

	return err
}
