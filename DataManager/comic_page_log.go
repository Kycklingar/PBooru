package DataManager

import "database/sql"

const lComicPage logtable = "comic_page"

func init() {
	logTableGetFuncs[lComicPage] = getLogComicPage
}

type logComicPages []logComicPage

func (l logComicPages) log(logID int, tx *sql.Tx) error {
	for _, lcp := range l {
		err := lcp.log(logID, tx)
		if err != nil {
			return err
		}
	}

	return nil
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
		SELECT action, comic_page_id, chapter_id, post_id, page,
			old_chapter_id, old_post_id, old_page
		FROM view_log_comic_page_diff
		WHERE log_id = $1
		`,
		log.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			lcp           logComicPage
			diffChapterID sql.NullInt32
			diffPostID    sql.NullInt32
			diffPage      sql.NullInt32
		)

		lcp.Post = NewPost()

		err = rows.Scan(
			&lcp.Action,
			&lcp.ID,
			&lcp.ChapterID,
			&lcp.Post.ID,
			&lcp.Page,
			&diffChapterID,
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
			lcp.Diff.ChapterID = int(diffChapterID.Int32)
		}

		log.ComicPages = append(log.ComicPages, lcp)
	}

	return nil
}
