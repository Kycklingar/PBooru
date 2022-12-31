package DataManager

import (
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/user"
)

func HasVoted(userID user.ID, postID int) (bool, error) {
	var res bool
	return res, db.Context.QueryRow(
		`SELECT EXISTS (
			SELECT *
			FROM post_score_mapping
			WHERE post_id = $1
			AND user_id = $2
		)`,
		postID,
		userID,
	).Scan(&res)
}

func RecentUploads(userID user.ID) ([]Post, error) {
	var posts []Post
	err := db.QueryRows(
		db.Context,
		`SELECT id
		FROM posts
		WHERE uploader = $1
		ORDER BY id DESC
		LIMIT 5`,
		userID,
	)(func(scan db.Scanner) error {
		var p = NewPost()
		err := scan(&p.ID)
		posts = append(posts, *p)

		return err
	})

	return posts, err
}

func RecentVotes(userID user.ID) ([]Post, error) {
	var posts []Post
	err := db.QueryRows(
		db.Context,
		`SELECT post_id
		FROM post_score_mapping
		WHERE user_id = $1
		ORDER BY id DESC
		LIMIT 5`,
		userID,
	)(func(scan db.Scanner) error {
		var p = NewPost()
		err := scan(&p.ID)
		posts = append(posts, *p)

		return err
	})

	return posts, err
}
