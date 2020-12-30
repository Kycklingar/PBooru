package DataManager

import (
	"database/sql"

	ts "github.com/kycklingar/PBooru/DataManager/timestamp"
)

type ForumThread struct {
	ReplyCount int
	Id         int
	Head       ForumPost
	Bumped     ts.Timestamp

	Posts []*ForumPost

	allPosts map[int]*ForumPost
}

func (thread *ForumThread) CompileBodies() {
	//var scatter = make(map[int]string)

	for _, p := range thread.allPosts {
		//compiledPosts[k] = p.CompileBody()
		p.CompileBody()
	}

	for i := range thread.Posts {
		thread.Posts[i].InsertRefs(thread.allPosts)
	}

}

func (thread *ForumThread) appendPost(p *ForumPost) {
	if _, ok := thread.allPosts[p.Id]; !ok {
		thread.allPosts[p.Id] = p
	}
}

func GetThread(board string, rid int) (*ForumThread, error) {
	thread, err := func() (*ForumThread, error) {
		var thread = new(ForumThread)
		thread.allPosts = make(map[int]*ForumPost)
		thread.Id = rid

		rows, err := DB.Query(`
			SELECT fp.rid, fp.title, fp.body, fp.created, fp.poster, users.username
			FROM forum_post fp
			LEFT JOIN users
			ON fp.poster = users.id
			WHERE fp.thread_id = (
				SELECT thread_id
				FROM forum_post p
				JOIN forum_thread t
				ON t.id = p.thread_id
				WHERE p.rid = $1
				AND t.board = $2
			)
			ORDER BY rid ASC
			`,
			rid,
			board,
		)
		if err != nil {
			return thread, err
		}
		defer rows.Close()

		for rows.Next() {
			var fp ForumPost
			var userid sql.NullInt64
			var username sql.NullString
			if err = rows.Scan(
				&fp.Id,
				&fp.Title,
				&fp.Body,
				&fp.Created,
				&userid,
				&username,
			); err != nil {
				return thread, err
			}

			if userid.Valid {
				fp.Poster = NewUser()
				fp.Poster.ID = int(userid.Int64)
				fp.Poster.Name = username.String
			}

			thread.Posts = append(thread.Posts, &fp)
			thread.appendPost(&fp)
		}

		return thread, nil
	}()
	if err != nil {
		return thread, err
	}

	//appendPosts := func(postmap map[int][]ForumPost) {
	//	for _, posts := range postmap {
	//		for i := range posts {
	//			if _, ok := thread.allPosts[posts[i].Id]; !ok {
	//				thread.allPosts[posts[i].Id] = &posts[i]
	//			}
	//		}
	//	}
	//}

	err = thread.getReferences(board, rid)
	if err != nil {
		return thread, err
	}

	err = thread.getReplies(board, rid)
	if err != nil {
		return thread, err
	}

	thread.CompileBodies()

	return thread, nil
}

func (thread *ForumThread) getReplies(board string, rid int) error {
	rows, err := DB.Query(`
		SELECT fp.rid, r.rid, r.thread_id, r.title, r.body, r.created, users.id, users.username
		FROM forum_post r
		LEFT JOIN users
		ON users.id = r.poster
		JOIN forum_replies rp
		ON r.id = rp.post_id
		JOIN forum_post fp
		ON fp.id = rp.ref
		WHERE fp.thread_id = (
			SELECT thread_id
			FROM forum_post p
			JOIN forum_thread t
			ON t.id = p.thread_id
			WHERE p.rid = $1
			AND t.board = $2
		)
		`,
		rid,
		board,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	//var replies = make(map[int][]ForumPost)

	for rows.Next() {
		var (
			key      int
			r        ForumPost
			userID   sql.NullInt64
			username sql.NullString
		)

		err = rows.Scan(&key, &r.Id, &r.Thread, &r.Title, &r.Body, &r.Created, &userID, &username)
		if err != nil {
			return err
		}

		if userID.Valid {
			r.Poster = NewUser()
			r.Poster.ID = int(userID.Int64)
			r.Poster.Name = username.String
		}

		thread.appendPost(&r)

		//var (
		//	rep []ForumPost
		//	ok  bool
		//)

		//if rep, ok = replies[key]; !ok {
		//	rep = []ForumPost{}
		//}

		//rep = append(rep, r)
		//replies[key] = rep

	}

	return nil
}

func (thread *ForumThread) getReferences(board string, rid int) error {
	rows, err := DB.Query(`
		SELECT fp.rid, r.rid, r.thread_id, r.title, r.body, r.created, users.id, users.username
		FROM forum_post r
		LEFT JOIN users
		ON users.id = r.poster
		JOIN forum_replies rp
		ON r.id = rp.ref
		JOIN forum_post fp
		ON fp.id = rp.post_id
		WHERE fp.thread_id = (
			SELECT thread_id
			FROM forum_post p
			JOIN forum_thread t
			ON t.id = p.thread_id
			WHERE p.rid = $1
			AND t.board = $2
		)
		`,
		rid,
		board,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	//var refs = make(map[int][]ForumPost)

	for rows.Next() {
		var (
			p        int
			r        ForumPost
			userID   sql.NullInt64
			username sql.NullString
		)
		err = rows.Scan(&p, &r.Id, &r.Thread, &r.Title, &r.Body, &r.Created, &userID, &username)
		if err != nil {
			return err
		}

		if userID.Valid {
			r.Poster = NewUser()
			r.Poster.ID = int(userID.Int64)
			r.Poster.Name = username.String
		}

		thread.appendPost(&r)

		//var (
		//	rep []ForumPost
		//	ok  bool
		//)

		//if rep, ok = refs[p]; !ok {
		//	rep = []ForumPost{}
		//}

		//rep = append(rep, r)
		//refs[p] = rep
	}

	return nil
}
