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

	var a func() error
	a = tx.Rollback

	defer func() {
		if err := a(); err != nil {
			log.Println(err)
		}
	}()

	// Handle conflicts
	if err = conflicts(tx, dupe); err != nil {
		return err
	}

	dup, err := getDupeFromPost(tx, dupe.Post)
	if err != nil {
		return err
	}

	dupe.Post = dup.Post

	if err = updateDupes(tx, dupe); err != nil {
		log.Println(err)
		return err
	}

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

	for _, p := range dupe.Inferior {
		if err = p.Delete(tx); err != nil {
			log.Println(err)
			return err
		}
	}

	// TODO
	// Move post descriptions and comments

	// Reset tag cache
	tc := new(TagCollector)
	tc.GetFromPost(tx, dupe.Post)
	for _, tag := range tc.Tags {
		resetCacheTag(tag.ID)
	}

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

func updateDupes(tx querier, dupe Dupe) error {
	for _, p := range dupe.Inferior {
		dup, err := getDupeFromPost(tx, p)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			UPDATE duplicates
			SET post_id = $1
			WHERE post_id = $2
			`,
			dupe.Post.ID,
			dup.Post.ID,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func conflicts(tx querier, dupe Dupe) error {
	in := func(dups []Dupe, p *Post) bool {
		for _, d := range dups {
			if d.Post.ID == p.ID {
				return true
			}
		}

		return false
	}

	var newSets []Dupe
	for _, p := range dupe.Inferior {
		dup, err := getDupeFromPost(tx, p)
		if err != nil {
			return err
		}

		if !in(newSets, dup.Post) {
			newSets = append(newSets, dup)
		}
	}

	superiors := func() bool {
		for _, p := range dupe.Inferior {
			for _, dupSet := range newSets {
				if dupSet.Post.ID == p.ID {
					return true
				}
			}
		}

		return false
	}

	bdupe, err := getDupeFromPost(tx, dupe.Post)
	if err != nil {
		return err
	}

	// There are two sets
	// A > B
	// D > E

	// Do these new sets work
	// B > C
	// Yes. Replace B with A > C
	//if bdupe.Post.ID != dupe.Post.ID && superiors() {
	//	return nil
	//}

	//// B > D
	//// Yes. Replace B with A > D
	//if bdupe.Post.ID != dupe.Post.ID && superiors() {
	//	return nil
	//}

	//// C > B
	//// No. Replace B with A, we don't know if C > A
	//if len(bdupe.Inferior) <= 0 && !superiors() {
	//	return error
	//}

	//// B > E
	//// No. Replace B with A and E with D, we don't know if A > D
	//if bdupe.Post.ID != dupe.Post.ID && !superiors() {
	//	return error
	//}

	// Will alway result in conflict
	if !superiors() {
		var derr DupeConflict

		derr.NeedChecking = append(derr.NeedChecking, bdupe.Post)

		for _, set := range newSets {
			derr.NeedChecking = append(derr.NeedChecking, set.Post)
		}

		return derr
	}

	// Don't want to assign a post to itself
	for _, set := range newSets {
		if set.Post.ID == bdupe.Post.ID {
			return errors.New("Conflicting dupe assignment. Trying to assign post as a dupe of itself")
		}
	}

	return nil
}

type DupeConflict struct {
	NeedChecking []*Post
}

func (d DupeConflict) Error() string {
	var str string
	for _, p := range d.NeedChecking {
		str += fmt.Sprint(p.ID, " ")
	}
	return fmt.Sprint("Duplicate conflict. Please compare: ", str)
}