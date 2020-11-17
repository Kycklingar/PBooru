package DataManager

import "database/sql"

type Thread struct {
	Replies int
	Post ForumPost
}

type ForumPost struct {
	Id int
	Title string
	Body string
	Poster *User
	Created timestamp
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

func GetCatalog(board string) ([]Thread, error) {
	rows, err := DB.Query(`
		SELECT rid, title, body, created, (
			SELECT count(*)
			FROM forum_post cfp
			WHERE cfp.reply_to = fp.id
		) as reply_count
		FROM forum_post fp
		JOIN forum_board fb
		ON fb.id = board_id
		WHERE reply_to IS NULL
		AND fb.uri = $1
		ORDER BY created DESC
		LIMIT 20
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
		err = rows.Scan(
			&t.Post.Id,
			&t.Post.Title,
			&t.Post.Body,
			&t.Post.Created,
			&t.Replies,
		)
		if err != nil {
			return nil, err
		}

		threads = append(threads, t)
	}

	return threads, nil
}

func GetThread(board string, thread int) ([]ForumPost, error) {
	rows, err := DB.Query(`
		SELECT rid, title, body, created, poster, users.username
		FROM forum_post
		JOIN forum_board fb
		ON fb.id = board_id
		LEFT JOIN users
		ON users.id = poster
		WHERE fb.uri = $1
		AND (
			reply_to = (
				SELECT fp.id
				FROM forum_post fp
				JOIN forum_board fb
				ON fb.id = fp.board_id
				WHERE rid = $2
				AND fb.uri = $1
			)
			OR rid = $2
		)
		ORDER BY rid ASC 
		`,
		board,
		thread,
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
		if err = rows.Scan(
			&fp.Id,
			&fp.Title,
			&fp.Body,
			&fp.Created,
			&userid,
			&username,
		); err != nil {
			return nil, err
		}

		if userid.Valid {
			fp.Poster = NewUser()
			fp.Poster.ID = int(userid.Int64)
			fp.Poster.Name = username.String
		}

		fps = append(fps, fp)
	}

	return fps, nil
}

// TODO: files
func NewForumPost(user *User, replyto *int, board, title, body string) (int, error) {
	var id, rid, top int

	tx, err := DB.Begin()
	if err != nil {
		return rid, err
	}
	defer commitOrDie(tx, &err)

	var poster *int
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
		return rid, err
	}

	err = tx.QueryRow(`
		INSERT INTO forum_post (board_id, title, body, reply_to, rid, poster)
		VALUES(
			(SELECT id FROM forum_board WHERE uri = $1),
			$2,
			$3,
			(
				SELECT fp.id
				FROM forum_post fp
				JOIN forum_board fb
				ON fb.id = board_id
				WHERE fb.uri = $1
				AND rid = $4
			),
			$5,
			$6
		)
		RETURNING id
		`,
		board,
		title,
		body,
		replyto,
		top,
		poster,
	).Scan(&id)
	if err != nil {
		return rid, err
	}

	// Return thread id
	err = tx.QueryRow(`
		SELECT COALESCE(
			(
				SELECT rid
				FROM forum_post
				WHERE id = (
					SELECT reply_to
					FROM forum_post
					WHERE id = $1
				)
			),
			(
				SELECT rid
				FROM forum_post
				WHERE id = $1
			)
		)`,
		id,
	).Scan(&rid)

	return rid, err
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
