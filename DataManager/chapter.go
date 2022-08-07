package DataManager

import (
	"database/sql"
	"fmt"

	mm "github.com/kycklingar/MinMax"
	"github.com/kycklingar/PBooru/DataManager/sqlbinder"
)

type Chapter struct {
	ID    int
	Title string
	Order int
	Pages []ComicPage
	Comic *Comic
}

func (c *Chapter) PageCount() int {
	return len(c.Pages)
}

func (c *Chapter) NPages(n int) []ComicPage {
	return c.Pages[:mm.Min(n, len(c.Pages))]
}

func (c *Chapter) getPages(q querier, limit int) error {
	var limitS string
	if limit > 0 {
		limitS = fmt.Sprintf("LIMIT %d", limit)
	}

	return query(
		q,
		fmt.Sprintf(`
			SELECT id, post_id, page
			FROM comic_page
			WHERE chapter_id = $1
			ORDER BY page ASC
			%s
			`,
			limitS,
		),
		c.ID,
	)(func(scan scanner) error {
		var cp = ComicPage{
			Post: NewPost(),
		}

		err := scan(&cp.ID, &cp.Post.ID, &cp.Page)
		if err != nil {
			return err
		}

		err = cp.Post.QMul(
			q,
			PFHash,
			PFThumbnails,
			PFDescription,
		)
		if err != nil {
			return err
		}

		c.Pages = append(c.Pages, cp)
		return nil
	})
}

func GetPostChapters(postID int, postFields ...sqlbinder.Field) ([]*Chapter, error) {
	chapters, err := postChapters(DB, postID)
	if noRows(err) != nil {
		return nil, err
	}

	for _, chapter := range chapters {
		err = chapter.getPages(DB, 5)
		if err != nil {
			return nil, err
		}
	}

	return chapters, nil
}

func postChapters(q querier, postID int) ([]*Chapter, error) {
	rows, err := DB.Query(`
		SELECT
			c.id, c.title,
			ch.id, ch.c_order, ch.title
		FROM comic_page cp
		JOIN comic_chapter ch
		ON cp.chapter_id = ch.id
		JOIN comics c
		ON c.id = ch.comic_id
		WHERE cp.post_id = $1
		`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chapters []*Chapter

	for rows.Next() {
		var chapter = new(Chapter)
		chapter.Comic = new(Comic)
		err = rows.Scan(
			&chapter.Comic.ID,
			&chapter.Comic.Title,
			&chapter.ID,
			&chapter.Order,
			&chapter.Title,
		)
		if err != nil {
			return nil, err
		}

		chapters = append(chapters, chapter)
	}

	return chapters, nil
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

		l.addTable(lChapter)
		l.fn = logChapter{
			Action:  aCreate,
			ComicID: comicID,
			ID:      id,
			Order:   order,
			Title:   title,
		}.log

		return
	}
}

func EditChapter(chapterID, order int, title string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var (
			comicID int
			pOrder  int
			pTitle  string
		)

		err = tx.QueryRow(`
			SELECT comic_id, c_order, title
			FROM comic_chapter
			WHERE id = $1
			`,
			chapterID,
		).Scan(&comicID, &pOrder, &pTitle)
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

		l.addTable(lChapter)
		l.fn = logChapter{
			Action:  aModify,
			ComicID: comicID,
			ID:      chapterID,
			Order:   order,
			Title:   title,
		}.log

		return
	}
}

func DeleteChapter(chapterID int, comicID *int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		// Remove all comic pages
		rows, err := tx.Query(`
			DELETE FROM comic_page
			WHERE chapter_id = $1
			RETURNING id, post_id, page
			`,
			chapterID,
		)
		if err != nil {
			return
		}
		defer rows.Close()

		var lc = logChapter{
			Action: aDelete,
			ID:     chapterID,
		}

		for rows.Next() {
			var lcp = logComicPage{
				Action:    aDelete,
				ChapterID: chapterID,
			}
			err = rows.Scan(
				&lcp.ID,
				&lcp.postID,
				&lcp.Page,
			)
			if err != nil {
				return
			}
			lcp.pids = []int{lcp.postID}
			lc.pages = append(lc.pages, lcp)
		}

		err = tx.QueryRow(`
			DELETE FROM comic_chapter
			WHERE id = $1
			RETURNING comic_id, c_order, title
			`,
			chapterID,
		).Scan(
			&lc.ComicID,
			&lc.Order,
			&lc.Title,
		)
		if err != nil {
			return
		}

		*comicID = lc.ComicID

		l.addTable(lChapter)
		if len(lc.pages) > 0 {
			l.addTable(lComicPage)
		}
		l.fn = lc.log

		return

	}
}

func ShiftChapterPages(chapterID, shiftBy, symbol, page int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var sym string

		switch symbol {
		case 1:
			sym = ">"
		default:
			sym = "<"
		}

		rows, err := tx.Query(
			fmt.Sprintf(`
				UPDATE comic_page
				SET page = page + $1
				WHERE chapter_id = $2
				AND page %s $3
				RETURNING id, post_id, page
				`,
				sym,
			),
			shiftBy,
			chapterID,
			page,
		)
		if err != nil {
			return
		}
		defer rows.Close()

		var lcps logComicPages

		for rows.Next() {
			var lcp = logComicPage{
				ChapterID: chapterID,
				Action:    aModify,
			}

			rows.Scan(
				&lcp.ID,
				&lcp.postID,
				&lcp.Page,
			)

			lcps = append(lcps, lcp)
		}

		l.addTable(lComicPage)
		l.fn = lcps.log

		return
	}
}
