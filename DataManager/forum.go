package DataManager

type Thread struct {
	Replies int
	Post ForumPost
}

type ForumPost struct {
	Id int
	Title string
	Body string
	Created timestamp
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
		SELECT rid, title, body, created
		FROM forum_post
		JOIN forum_board fb
		ON fb.id = board_id
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
		if err = rows.Scan(
			&fp.Id,
			&fp.Title,
			&fp.Body,
			&fp.Created,
		); err != nil {
			return nil, err
		}

		fps = append(fps, fp)
	}

	return fps, nil
}

func NewForumPost(replyto *int, board, title, body string) (int, error) {
	var id, rid int

	//n := func() interface{}{
	//	if replyto != nil {
	//		return *replyto
	//	}
	//	return nil
	//}

	//fmt.Println(n())
	err := DB.QueryRow(`
		INSERT INTO forum_post (board_id, title, body, reply_to, rid)
		VALUES(
			(SELECT id FROM forum_board WHERE uri = $1),
			$2,
			$3,
			(SELECT fp.id FROM forum_post fp JOIN forum_board fb ON fb.id = board_id WHERE fb.uri = $1 AND rid = $4),
			(SELECT COALESCE(MAX(rid), 0) + 1 FROM forum_post JOIN forum_board fb ON board_id = fb.id WHERE fb.uri = $1)
		)
		RETURNING id
		`,
		board,
		title,
		body,
		replyto,
	).Scan(&id)

	err = DB.QueryRow(`
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
