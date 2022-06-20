package DataManager

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/kycklingar/PBooru/DataManager/timestamp"
)

// CommentModel is used to retriev and save comments
type CommentCollector struct {
	Comments []*Comment
}

type Comment struct {
	ID           int
	User         *User
	Text         string
	CompiledText string
	Time         timestamp.Timestamp
}

// Initialize a new comment
func NewComment() *Comment {
	return &Comment{User: NewUser()}
}

func CommentByID(id int) (*Comment, error) {
	var c = NewComment()
	c.ID = id

	var uID *int

	err := DB.QueryRow(`
		SELECT user_id, text, timestamp
		FROM comment_wall
		WHERE id = $1`,
		id,
	).Scan(&uID, &c.Text, &c.Time)

	if uID != nil {
		c.User.ID = *uID
	}

	return c, err
}

// Get the latest comments
func (cm *CommentCollector) Get(q querier, count int, daemon string) error {
	rows, err := q.Query("SELECT id, user_id, text, timestamp FROM comment_wall ORDER BY id DESC LIMIT $1", count)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		c := NewComment()
		var userID *int
		err = rows.Scan(&c.ID, &userID, &c.Text, &c.Time)
		if err != nil {
			return err
		}
		if userID != nil {
			c.User.ID = *userID
			c.User = CachedUser(c.User)
		}
		cm.Comments = append(cm.Comments, c)
	}

	for i := range cm.Comments {
		cm.Comments[i].CompiledText = compileBBCode(q, cm.Comments[i].Text, daemon)
	}

	return rows.Err()
}

func verifyCommentPosts(text string) error {
	if suc, err := regexp.MatchString("\\[post(?s).*\\]", text); err != nil || suc {
		if err != nil {
			log.Println(err)
			return err
		}
		//return fmt.Errorf("Post does not exist")

		reg, err := regexp.Compile("\\[post=([0-9]+)]")
		if err != nil {
			log.Println(err)
			return err
		}

		res := reg.FindAllStringSubmatch(text, -1)

		if len(res) <= 0 {
			return fmt.Errorf("Post does not exist")
		}

		for _, val := range res {
			post := NewPost()
			id, err := strconv.Atoi(val[1])
			if err != nil {
				log.Println("Post id is not a number. This should never happen.", err)
				return err
			}
			if err = post.SetID(DB, id); err != nil {
				return fmt.Errorf("Post does not exist")
			}
		}
	}

	return nil
}

func (cm Comment) Username() string {
	if cm.User.ID <= 0 {
		return "Anonymous"
	}

	return cm.User.Name
}

func DeleteComment(id int) error {
	_, err := DB.Exec(`
		DELETE FROM comment_wall
		WHERE id = $1
		`,
		id,
	)

	return err
}

//Save a new comment to the wall
func (cm *Comment) Save(userID int, text string) error {

	if err := verifyCommentPosts(text); err != nil {
		return err
	}

	isNull := func(id int) *int {
		if id == 0 {
			return nil
		}
		return &id
	}
	_, err := DB.Exec("INSERT INTO comment_wall(user_id, text) VALUES($1, $2)", isNull(userID), strings.TrimSpace(text))
	return err
}

func (cm *Comment) Edit(text string) error {
	if err := verifyCommentPosts(text); err != nil {
		return err
	}

	_, err := DB.Exec("UPDATE comment_wall SET text = $1 WHERE id = $2", text, cm.ID)
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
	Time timestamp.Timestamp
}

// Save a new comment on a post
func (pc *PostComment) Save(q querier) error {
	if pc.Text == "" || pc.Post.ID == 0 || pc.User.QID(q) == 0 {
		return fmt.Errorf("expected: Text, PostID, UserID. Got: %s, %d, %d", pc.Text, pc.Post.ID, pc.User.ID)
	}

	_, err := DB.Exec("INSERT INTO post_comments(post_id, user_id, text) VALUES($1, $2, $3)", pc.Post.ID, pc.User.ID, pc.Text)

	return err
}
