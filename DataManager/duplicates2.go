package DataManager

import (
	"errors"
	"fmt"
	"log"
)

type Dupe struct {
	Post     *Post
	Inferior []*Post
}

func getDupeFromPost(q querier, p *Post) (Dupe, error) {
	var dup Dupe
	dup.Post = p
	rows, err := q.Query(`
		SELECT post_id, dup_id
		FROM duplicates
		WHERE post_id = $1
		OR post_id = (
			SELECT post_id
			FROM duplicates
			WHERE dup_id = $1
		)
		`,
		p.ID,
	)
	if err != nil {
		return dup, err
	}
	defer rows.Close()

	for rows.Next() {
		var np = NewPost()
		var bp = NewPost()

		err = rows.Scan(&bp.ID, &np.ID)
		if err != nil {
			return dup, err
		}

		if dup.Post.ID != bp.ID {
			dup.Post = bp
		}
		dup.Inferior = append(dup.Inferior, np)
	}

	return dup, nil
}

func AssignDuplicates(dupe Dupe, user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	fmt.Println(dupe)

	var a func() error
	a = tx.Rollback

	defer func() {
		if err := a(); err != nil {
			log.Println(err)
		}
	}()

	// TODO:
	// Handle conflicts

	// set (a, b) merge set (c, b)
	// we don't know wheter c is better than a
	// require new comparison of a and c

	// set (a, b) merge set (b, c)
	// we do know a is better than b and c
	// go ahead

	// confclicts(tx, dupe)

	var values string
	for _, p := range dupe.Inferior {
		if values != "" {
			values += ","
		}
		values += fmt.Sprintf("(%d, %d)", dupe.Post.ID, p.ID)
	}

	var query = fmt.Sprint("INSERT INTO duplicates(post_id, dup_id) VALUES", values)
	_, err = tx.Exec(query)
	if err != nil {
		log.Println(err)
		return err
	}

	// For each inferior post, move tags to superior
	if err = moveTags(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Replace comics with new post
	if err = replaceComicPages(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Move votes to the new post
	if err = moveVotes(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// Move user pool posts
	if err = movePoolPosts(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

	// TODO
	// Move post descriptions and comments

	a = tx.Commit

	return nil
}

func movePoolPosts(tx querier, dupe Dupe) (err error) {
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			UPDATE pool_mappings
			SET post_id = $1
			WHERE post_id = $2
			AND pool_id NOT IN(
				SELECT pool_id
				FROM pool_mappings
				WHERE post_id = $1
			)
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}

		_, err = tx.Exec(`
			DELETE FROM pool_mappings
			WHERE post_id = $1
			`,
			p.ID,
		)
		if err != nil {
			return
		}
	}

	return
}

func moveVotes(tx querier, dupe Dupe) (err error) {
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			UPDATE post_score_mapping
			SET post_id = $1
			WHERE post_id = $2
			AND user_id NOT IN(
				SELECT user_id
				FROM post_score_mapping
				WHERE post_id = $1
			)
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}

		_, err = tx.Exec(`
			DELETE FROM post_score_mapping
			WHERE post_id = $1
			`,
			p.ID,
		)
		if err != nil {
			return err
		}
	}

	return
}

func moveTags(tx querier, dupe Dupe) (err error) {
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			INSERT INTO post_tag_mappings (post_id, tag_id)
				SELECT $1, tag_id FROM post_tag_mappings
				WHERE post_id = $2
			ON CONFLICT DO NOTHING
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}

		_, err = tx.Exec(`
			DELETE FROM post_tag_mappings
			WHERE post_id = $1
			`,
			p.ID,
		)
		if err != nil {
			return
		}
	}

	return
}

func replaceComicPages(tx querier, dupe Dupe) (err error) {
	for _, p := range dupe.Inferior {
		_, err = tx.Exec(`
			UPDATE comic_mappings
			SET post_id = $1
			WHERE post_id = $2
			`,
			dupe.Post.ID,
			p.ID,
		)
		if err != nil {
			return
		}
	}

	return
}

func conflicts(tx querier, dupe Dupe) error {
	var vals string
	for _, p := range dupe.Inferior {
		vals += fmt.Sprint(p.ID, ",")
	}
	vals += fmt.Sprint(dupe.Post.ID)

	query := fmt.Sprintf(`
		SELECT post_id
		FROM duplicates
		WHERE dup_id IN(%s)
		`,
		vals,
	)

	rows, err := tx.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var ids []int

	for rows.Next() {
		var id int
		if err = rows.Scan(&id); err != nil {
			return err
		}

		in := func(id int) bool {
			for _, i := range ids {
				if i == id {
					return true
				}
			}

			return false
		}

		if !in(id) {
			ids = append(ids, id)
		}
	}

	// Potential conflict
	if len(ids) == 1 {
		return errors.New("ASD")
	}

	return nil

}
