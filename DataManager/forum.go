package DataManager

import (
	"database/sql"
	"strings"
	"fmt"
	"strconv"
	"github.com/kycklingar/PBooru/DataManager/forum"
)

const bumpLimit = 300

type Thread struct {
	Replies int
	Post ForumPost
	Bumped timestamp
}

type ForumPost struct {
	Id int
	Title string
	Body forum.Body
	Poster *User
	Created timestamp
	Backlinks []int
	//Replies []ForumPost
	//Refs []ForumPost
}

type Board struct {
	Name string
	Description string
	Uri string
}

func GetCategories() ([]string, error) {
	rows, err := DB.Query(`
		SELECT name
		FROM forum_category
		`,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var cats []string

	for rows.Next() {
		var c string
		err = rows.Scan(&c)
		if err != nil {
			return nil, err
		}

		cats = append(cats, c)
	}

	return cats, nil
}

func GetBoards() (map[string][]Board, error) {
	rows, err := DB.Query(`
		SELECT cat.name, b.name, b.description, b.uri
		FROM forum_board b 
		JOIN forum_category cat
		ON cat.id = b.category
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards = make(map[string][]Board)

	for rows.Next() {
		var (
			b Board
			cat string
		)

		if err = rows.Scan(&cat, &b.Name, &b.Description, &b.Uri); err != nil {
			return nil, err
		}

		boards[cat] = append(boards[cat], b)
	}

	return boards, nil
}

func GetBoard(board string) (b Board, err error) {
	err = DB.QueryRow(`
		SELECT name, description, uri
		FROM forum_board
		WHERE uri = $1
		`,
		board,
	).Scan(&b.Name, &b.Description, &b.Uri)

	return
}

func GetCatalog(board string) ([]Thread, error) {
	rows, err := DB.Query(`
		SELECT rid, title, body, created, bumped, users.username, (
			SELECT count(*)
			FROM forum_post cp
			WHERE cp.thread_id = t.id
		) as reply_count
		FROM forum_thread t
		JOIN forum_post p
		ON t.start_post = p.id
		JOIN users
		ON users.id = p.poster
		WHERE board = $1
		ORDER BY bumped DESC
		`,
		board,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread

	for rows.Next() {
		var t Thread
		var username sql.NullString
		err = rows.Scan(
			&t.Post.Id,
			&t.Post.Title,
			&t.Post.Body,
			&t.Post.Created,
			&t.Bumped,
			&username,
			&t.Replies,
		)
		if err != nil {
			return nil, err
		}

		if username.Valid {
			t.Post.Poster = NewUser()
			t.Post.Poster.Name = username.String
		}

		threads = append(threads, t)
	}

	return threads, nil
}

func GetThread(board string, rid int) ([]ForumPost, error) {
	posts, err := func() ([]ForumPost, error) {
		rows, err := DB.Query(`
			SELECT fp.rid, fp.title, fp.body, fp.created, fp.poster, users.username, array_agg(re.rid)
			FROM forum_post fp
			LEFT JOIN forum_replies r
			ON r.ref = fp.id
			LEFT JOIN forum_post re
			ON r.post_id = re.id
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
			GROUP BY fp.rid, fp.title, fp.body, fp.created, fp.poster, users.username
			ORDER BY rid ASC
			`,
			rid,
			board,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var fps []ForumPost

		for rows.Next() {
			var fp ForumPost
			var userid sql.NullInt64
			var username sql.NullString
			var backlinks string
			if err = rows.Scan(
				&fp.Id,
				&fp.Title,
				&fp.Body,
				&fp.Created,
				&userid,
				&username,
				&backlinks,
			); err != nil {
				return nil, err
			}

			for _, val := range strings.Split(backlinks[1:len(backlinks)-1], ",") {
				i, err := strconv.Atoi(val)
				if err == nil {
					fp.Backlinks = append(fp.Backlinks, i)
				}
			}

			if userid.Valid {
				fp.Poster = NewUser()
				fp.Poster.ID = int(userid.Int64)
				fp.Poster.Name = username.String
			}

			fps = append(fps, fp)
		}

		return fps, nil
	}()
	if err != nil {
		return nil, err
	}

	//replies, err := getReferences(board, rid)
	//if err != nil {
	//	return nil, err
	//}

	//for k, v := range replies {
	//	for i := range posts {
	//		if posts[i].Id == k {
	//			posts[i].Replies = append(posts[i].Replies, v...)
	//			break
	//		}

	//		for _, r := range v {
	//			if posts[i].Id == r.Id {
	//				posts[i].Refs = append(posts[i].Refs, r)
	//			}
	//		}
	//	}
	//}


	return posts, nil
}

func getReferences(board string, rid int) (map[int][]ForumPost, error) {
	rows, err := DB.Query(`
		SELECT fp.rid, r.rid, r.title, r.body, r.created, users.id, users.username
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
		return nil, err
	}
	defer rows.Close()

	var replies = make(map[int][]ForumPost)

	for rows.Next() {
		var (
			p int
			r ForumPost
			userID sql.NullInt64
			username sql.NullString
		)
		err = rows.Scan(&p, &r.Id, &r.Title, &r.Body, &r.Created, &userID, &username)
		if err != nil {
			return nil, err
		}

		if userID.Valid {
			r.Poster = NewUser()
			r.Poster.ID = int(userID.Int64)
			r.Poster.Name = username.String
		}

		var (
			rep []ForumPost
			ok bool
		)

		if rep, ok = replies[p]; !ok {
			rep = []ForumPost{}
		}

		rep = append(rep, r)
		replies[p] = rep
	}

	fmt.Println(replies)

	return replies, nil
}

func DeleteForumPost(board string, rid int) error {
	_, err := DB.Exec(`
		DELETE FROM forum_post
		WHERE id = (
			SELECT fp.id
			FROM forum_post fp
			JOIN forum_thread ft
			ON fp.thread_id = ft.id
			WHERE ft.board = $1
			AND fp.rid = $2
		)
		`,
		board,
		rid,
	)

	return err
}

func NewThread(board, title, body string, user *User) (int, error) {
	var (
		threadID int
		pid int
		rid int
	)

	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	defer commitOrDie(tx, &err)

	err = tx.QueryRow(`
		INSERT INTO forum_thread(board, bump_limit)
		VALUES(
			$1,
			$2
		)
		RETURNING id
		`,
		board,
		bumpLimit,
	).Scan(&threadID)
	if err != nil {
		return 0, err
	}

	pid, err = postTX(tx, threadID, board, title, body, user)
	if err != nil {
		return pid, err
	}

	_, err = tx.Exec(`
		UPDATE forum_thread
		SET start_post = $1
		WHERE id = $2
		`,
		pid,
		threadID,
	)
	if err != nil {
		return rid, err
	}

	err = tx.QueryRow(`
		SELECT rid
		FROM forum_post
		WHERE id = $1
		`,
		pid,
	).Scan(&rid)
	if err != nil {
		return rid, err
	}

	err = forum.PruneBoard(tx, board)

	return rid, err
}

// TODO: files
func ForumReply(replyID int, board, title, body string, user *User) (int, error) {
	var (
		rid int
		threadID int
	)

	tx, err := DB.Begin()
	if err != nil {
		return rid, err
	}
	defer commitOrDie(tx, &err)

	tx.QueryRow(`
		SELECT thread_id
		FROM forum_post
		JOIN forum_thread t
		ON t.id = thread_id
		WHERE rid = $1
		AND board = $2
		`,
		replyID,
		board,
	).Scan(&threadID)

	_, err = postTX(tx, threadID, board, title, body, user)
	if err != nil {
		return rid, err
	}

	err = forum.Bump(tx, threadID)

	return replyID, err
}

func postTX(tx querier, thread int, board, title, body string, user *User) (int, error) {
	var (
		id int
		top int
		poster *int
		err error
	)

	if user.QID(tx) != 0 {
		poster = &user.ID
	}

	err = tx.QueryRow(`
		UPDATE forum_board
		SET top = top + 1
		WHERE uri = $1
		RETURNING top
		`,
		board,
	).Scan(&top)
	if err != nil {
		return id, err
	}

	err = tx.QueryRow(`
		INSERT INTO forum_post (thread_id, rid, poster, title, body)
		VALUES(
			$1,
			$2,
			$3,
			$4,
			$5
		)
		RETURNING id
		`,
		thread,
		top,
		poster,
		title,
		body,
	).Scan(&id)


	//References
	bod := forum.Body(body)

	for _, ref := range bod.Mentions() {
		_, err = tx.Exec(`
			INSERT INTO forum_replies (post_id, ref)
			VALUES($1, (
				SELECT p.id
				FROM forum_post p
				JOIN forum_thread t
				ON p.thread_id = t.id
				WHERE t.board = $2
				AND p.rid = $3
				)
			)
			`,
			id,
			board,
			ref,
		)
		if err != nil {
			return id, err
		}
	}

	return id, err
}

func NewBoard(uri, name, description, category string) error {
	_, err := DB.Exec(`
		INSERT INTO forum_board (
			uri,
			name,
			description,
			category
		)
		VALUES(
			$1,
			$2,
			$3,
			(
				SELECT id
				FROM forum_category
				WHERE name = $4
			)
		)
		`,
		uri,
		name,
		description,
		category,
	)

	return err
}

func NewCategory(name string) error {
	_, err := DB.Exec(`
		INSERT INTO forum_category (name)
		VALUES($1)
		`,
		name,
	)

	return err
}
