package DataManager

import "database/sql"

type ComicPage struct {
	ID   int
	Post *Post
	Page int
}

func CreateComicPage(chapterID, postID, page int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var id int

		err = tx.QueryRow(`
			INSERT INTO comic_page (
				chapter_id,
				post_id,
				page
			)
			VALUES($1, $2, $3)
			RETURNING id
			`,
			chapterID,
			postID,
			page,
		).Scan(&id)
		if err != nil {
			return
		}

		l.addTable(lComicPage)
		l.fn = logComicPage{
			Action:    aCreate,
			ID:        id,
			ChapterID: chapterID,
			postID:    postID,
			Page:      page,
			pids:      []int{postID},
		}.log

		return
	}
}

func EditComicPage(pageID, chapterID, postID, page int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var (
			pChapterID int
			pPostID    int
			pPage      int
		)

		err = tx.QueryRow(`
			SELECT chapter_id, post_id, page
			FROM comic_page
			WHERE id = $1
			`,
			pageID,
		).Scan(&pChapterID, &pPostID, &pPage)
		if err != nil {
			return
		}

		// Nothing to do
		if chapterID == pChapterID && pPostID == postID && pPage == page {
			return
		}

		_, err = tx.Exec(`
			UPDATE comic_page
			SET chapter_id = $1,
			post_id = $2,
			page = $3
			WHERE id = $4
			`,
			chapterID,
			postID,
			page,
			pageID,
		)
		if err != nil {
			return
		}

		l.addTable(lComicPage)
		l.fn = logComicPage{
			Action:    aModify,
			ID:        pageID,
			ChapterID: chapterID,
			postID:    postID,
			Page:      page,
			pids:      []int{postID, pPostID},
		}.log

		return
	}
}

func DeleteComicPage(pageID int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var lcp = logComicPage{
			Action: aDelete,
			ID:     pageID,
		}

		err = tx.QueryRow(`
			DELETE FROM comic_page
			WHERE id = $1
			RETURNING chapter_id, post_id, page
			`,
			pageID,
		).Scan(
			&lcp.ChapterID,
			&lcp.postID,
			&lcp.Page,
		)
		if err != nil {
			return
		}

		lcp.pids = []int{lcp.postID}
		l.addTable(lComicPage)
		l.fn = lcp.log

		return
	}
}
