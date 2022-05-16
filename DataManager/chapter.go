package DataManager

import (
	"database/sql"
	"errors"
)

type Chapter struct {
	ID    int
	Title string
	Order int
	Pages []ComicPage
}

func (c *Chapter) PageCount() int {
	return len(c.Pages)
}

func (c *Chapter) NPages(n int) []ComicPage {
	return c.Pages[:max(n, len(c.Pages))]
}

func (c *Chapter) getPages(q querier) error {
	rows, err := q.Query(`
		SELECT id, post_id, page
		FROM comic_page
		WHERE chapter_id = $1
		ORDER BY page ASC
		`,
		c.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cp = ComicPage{
			Post: NewPost(),
		}

		err = rows.Scan(&cp.ID, &cp.Post.ID, &cp.Page)
		if err != nil {
			return err
		}

		err = cp.Post.QMul(
			q,
			PFHash,
			PFThumbnails,
		)
		if err != nil {
			return err
		}

		c.Pages = append(c.Pages, cp)
	}

	return nil
}

func GetPostChapters(postID int) {

}

func CreateChapter(comicID, order int, title string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var id int
		err = tx.QueryRow(`
			INSERT INTO comic_chapter (
				comic_id,
				c_order,
				title

			)
			VALUES($1, $2, $3)
			RETURNING id
			`,
			comicID,
			order,
			title,
		).Scan(&id)
		if err != nil {
			return
		}

		l.table = lChapter
		l.fn = logChapter{
			Action: aCreate,
			ID:     id,
			Order:  order,
			Title:  title,
		}.log

		return
	}
}

func EditChapter(chapterID, order int, title string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var (
			pOrder int
			pTitle string
		)

		err = tx.QueryRow(`
			SELECT c_order, title
			FROM comic_chapter
			WHERE id = $1
			`,
			chapterID,
		).Scan(&pOrder, &pTitle)
		if err != nil {
			return
		}

		// Nothing to do
		if pOrder == order && pTitle == title {
			return
		}

		_, err = tx.Exec(`
			UPDATE comic_chapter
			SET c_order = $1,
			title = $2
			WHERE id = $3
			`,
			order,
			title,
			chapterID,
		)
		if err != nil {
			return
		}

		l.table = lChapter
		l.fn = logChapter{
			Action: aModify,
			ID:     chapterID,
			Order:  order,
			Title:  title,
		}.log

		return
	}
}

func DeleteChapter(chapterID int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		err = errors.New("not implemented")
		return
	}
}
