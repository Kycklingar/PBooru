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

	Diff *logComicPage

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
		SELECT p1.action, p1.comic_page_id, p1.chapter_id, p1.post_id, p1.page,
			p2.post_id, p2.page
		FROM log_comic_page p1
		LEFT JOIN log_comic_page p2
		ON p1.comic_page_id = p2.comic_page_id
		AND p2.log_id = (
			SELECT MAX(log_id)
			FROM log_comic_page
			WHERE log_id < p1.log_id
			AND comic_page_id = p1.comic_page_id
		)
		WHERE p1.log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			lcp        logComicPage
			diffPostID sql.NullInt32
			diffPage   sql.NullInt32
		)

		lcp.Post = NewPost()

		err = rows.Scan(
			&lcp.Action,
			&lcp.ID,
			&lcp.ChapterID,
			&lcp.Post.ID,
			&lcp.Page,
			&diffPostID,
			&diffPage,
		)
		if err != nil {
			return err
		}

		if lcp.Action == aModify && diffPostID.Valid {
			lcp.Diff = &logComicPage{
				Post: NewPost(),
				Page: int(diffPage.Int32),
			}
			lcp.Diff.Post.ID = int(diffPostID.Int32)
		}

		log.ComicPages = append(log.ComicPages, lcp)
	}

	return nil
}
