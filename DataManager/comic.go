package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/kycklingar/sqhell/cond"
)

type Comics struct {
	Comics []*Comic
	Total  int
}

type Comic struct {
	ID        int
	Title     string
	FrontPage *Post
	Chapters  []*Chapter
	PageCount int

	TagSummary []Tag
}

func SearchComics(title, tagStr string, limit, offset int) (c Comics, err error) {
	tags, err := tsChain(parseTags(tagStr)).qids(DB).aliases(DB).unwrap()
	if err != nil {
		return
	}

	var (
		tjoin  = new(cond.Group)
		gwhere = new(cond.Group)
		where  string
		glimit = new(cond.Group)
		params []interface{}
		pi     = 1
	)

	for i, tag := range tags {
		where = "WHERE "
		tjoin.Add("\n", cond.N(fmt.Sprintf("JOIN post_tag_mappings ptm%d", i))).
			Add("\n", cond.N(fmt.Sprintf("ON cm.post_id = ptm%d.post_id\n", i)))

		gwhere.Add("\nAND", cond.N(fmt.Sprintf("ptm%d.tag_id = %d\n", i, tag.ID)))
	}

	if len(title) > 0 {
		where = "WHERE "
		var o = new(int)
		gwhere.Add(
			"\nAND",
			cond.Wrap{
				Cond: new(cond.Group).Add(
					"\n",
					cond.O{
						S: "lower(c.title) LIKE '%%' || $%d || '%%'",
						I: o,
					},
				).Add(
					"\nOR",
					cond.O{
						S: "lower(cc.title) like '%%' || $%d || '%%'",
						I: o,
					},
				),
			},
		)

		params = append(params, title)
	}

	where = fmt.Sprintf(
		"%s%s",
		where,
		gwhere.Eval(&pi),
	)

	query := fmt.Sprintf(`
		FROM comics c
		JOIN comic_chapter cc
		ON c.id = cc.comic_id
		JOIN comic_page cm
		ON cc.id = cm.chapter_id
		%s
		%s
		`,
		tjoin.Eval(nil),
		where,
	)

	err = DB.QueryRow(`
		SELECT count(DISTINCT c.id)
		`+query,
		params...,
	).Scan(&c.Total)
	if err != nil {
		return
	}

	glimit.Add("", cond.P("LIMIT $%d")).
		Add("", cond.P("OFFSET $%d"))

	rows, err := DB.Query(
		fmt.Sprintf(`
			SELECT DISTINCT c.id, c.title, c.modified
			%s
			ORDER BY c.modified DESC
			%s`,
			query,
			glimit.Eval(&pi),
		),
		append(params, limit, offset)...,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			comic = new(Comic)
			trash string
		)

		err = rows.Scan(&comic.ID, &comic.Title, &trash)
		if err != nil {
			return
		}

		err = comic.fetch()
		if err != nil {
			return
		}

		c.Comics = append(c.Comics, comic)
	}

	return
}

func GetComic(comicID int) (*Comic, error) {
	var c = new(Comic)
	c.ID = comicID

	return c, c.fetch()
}

func (comic *Comic) ChapterIndex(index int) *Chapter {
	for _, chapter := range comic.Chapters {
		if chapter.Order == index {
			return chapter
		}
	}

	return nil
}

func (comic *Comic) fetch() error {
	err := DB.QueryRow(`
		SELECT title
		FROM comics
		WHERE id = $1
		`,
		comic.ID,
	).Scan(&comic.Title)
	if err != nil {
		return err
	}

	err = comic.tagSummary(DB)
	if err != nil {
		return err
	}

	err = comic.getChapters()
	if err != nil {
		return err
	}

	for _, chap := range comic.Chapters {
		if err = chap.getPages(DB, 0); err != nil {
			return err
		}
		if comic.FrontPage == nil && chap.PageCount() > 0 {
			comic.FrontPage = chap.Pages[0].Post
		}
		comic.PageCount += chap.PageCount()
	}

	return nil
}

func (comic *Comic) getChapters() error {
	rows, err := DB.Query(`
		SELECT id, title, c_order
		FROM comic_chapter
		WHERE comic_id = $1
		ORDER BY c_order ASC
		`,
		comic.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var c = new(Chapter)
		err = rows.Scan(&c.ID, &c.Title, &c.Order)
		if err != nil {
			return err
		}

		comic.Chapters = append(comic.Chapters, c)
	}

	return nil
}

func (comic *Comic) tagSummary(q querier) error {
	rows, err := q.Query(`
		SELECT t.tag, n.nspace
		FROM post_tag_mappings ptm
		JOIN comic_page cm
			ON ptm.post_id = cm.post_id
		JOIN comic_chapter cc
			ON cm.chapter_id = cc.id
		JOIN comics c
			ON cc.comic_id = c.id
		JOIN tags t
			ON ptm.tag_id = t.id
		JOIN namespaces n
			ON t.namespace_id = n.id
		WHERE c.id = $1
		GROUP BY t.tag, n.nspace ORDER BY count(ptm.tag_id) DESC LIMIT 15;
		`,
		comic.ID,
	)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t Tag
		err = rows.Scan(&t.Tag, &t.Namespace)
		if err != nil {
			return err
		}

		comic.TagSummary = append(comic.TagSummary, t)
	}

	return rows.Err()
}

func CreateComic(title string, retid *int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var id int
		err = tx.QueryRow(`
			INSERT INTO comics(title)
			VALUES($1)
			RETURNING id
			`,
			title,
		).Scan(&id)
		if err != nil {
			return
		}

		*retid = id

		l.addTable(lComic)
		l.fn = logComic{
			Action: aCreate,
			ID:     id,
			Title:  title,
		}.log

		return
	}
}

func EditComic(comicID int, title string) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		if len(title) <= 0 {
			err = errors.New("invalid comic title")
			return
		}

		var oldTitle string
		err = tx.QueryRow(`
			SELECT title
			FROM comics
			WHERE id = $1
			`,
			comicID,
		).Scan(&oldTitle)

		if oldTitle == title {
			return
		}

		_, err = tx.Exec(`
			UPDATE comics
			SET title = $1
			WHERE id = $2
			`,
			title,
			comicID,
		)
		if err != nil {
			return
		}

		l.addTable(lComic)
		l.fn = logComic{
			Action: aModify,
			ID:     comicID,
			Title:  title,
		}.log
		return
	}
}

func DeleteComic(comicID int) loggingAction {
	return func(tx *sql.Tx) (l logger, err error) {
		var title string
		err = tx.QueryRow(`
			DELETE FROM comics
			WHERE id = $1
			RETURNING title
			`,
			comicID,
		).Scan(&title)

		l.addTable(lComic)
		l.fn = logComic{
			Action: aDelete,
			Title:  title,
			ID:     comicID,
		}.log

		return
	}
}
