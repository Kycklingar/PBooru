package DataManager

import "database/sql"

const lComicPage logtable = "comic_page"

func init() {
	logTableGetFuncs[lComicPage] = getLogComicPage
}

type logComicPage struct {
	Action    lAction
	ID        int
	Post      *Post
	ChapterID int
	Page      int

	postID int
	pids   []int
}

func (l logComicPage) log(logID int, tx *sql.Tx) error {
	err := logAffectedPosts(logID, tx, l.pids)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO log_comic_page (
			log_id,
			action,
			comic_page_id,
			chapter_id,
			post_id,
			page
		)
		VALUES($1, $2, $3, $4, $5, $6)
		`,
		logID,
		l.Action,
		l.ID,
		l.ChapterID,
		l.postID,
		l.Page,
	)

	return err
}

func getLogComicPage(log *Log, q querier) error {
	rows, err := q.Query(`
		SELECT action, comic_page_id, chapter_id, post_id, page
		FROM log_comic_page
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var lcp logComicPage
		lcp.Post = NewPost()

		err = rows.Scan(
			&lcp.Action,
			&lcp.ID,
			&lcp.ChapterID,
			&lcp.Post.ID,
			&lcp.Page,
		)
		if err != nil {
			return err
		}

		log.ComicPages = append(log.ComicPages, lcp)
	}

	return nil
}
