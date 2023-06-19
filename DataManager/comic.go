package DataManager

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/postSearch"
	"github.com/kycklingar/PBooru/DataManager/search"
	"github.com/kycklingar/PBooru/DataManager/search/condition"
	"github.com/kycklingar/PBooru/DataManager/search/order"
	"github.com/kycklingar/PBooru/DataManager/search/table"
)

var comicTagSummaryCache = cache.NewGeneric[int, []Tag](nil, time.Minute*30, time.Hour*2)

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
		comics     = table.New("comics", "c")
		chapter    = table.New("comic_chapter", "ch")
		page       = table.New("comic_page", "cp")
		parameters search.Parameters
	)

	comics.Join(chapter).Fields(
		comics.Field("id"),
		chapter.Field("comic_id"),
	)
	comics.Join(page).Fields(
		chapter.Field("id"),
		page.Field("chapter_id"),
	)

	for i, tag := range tags {
		var ptm = postSearch.NewPtm(fmt.Sprint("ptm", i), tag.ID)
		comics.Join(ptm).Fields(
			page.Field("post_id"),
			ptm.Field("post_id"),
		)

		comics.Where(ptm)
	}

	if len(title) > 0 {
		var p = parameters.New(title)

		const format = "lower(%s) LIKE '%%' || %s || '%%'"
		var comicTitleCondition = condition.String(fmt.Sprintf(format, comics.Field("title"), p))
		var chapterTitleCondition = condition.String(fmt.Sprintf(format, chapter.Field("title"), p))

		var g = condition.Group(comicTitleCondition)
		g.Or(chapterTitleCondition)

		comics.Where(g)
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf("SELECT count(DISTINCT %s)", comics.Field("id")))
	b.WriteString("\nFROM ")
	comics.Eval(&b)
	err = DB.QueryRow(b.String(), parameters...).Scan(&c.Total)
	if err != nil {
		return
	}

	var o = order.By(comics.Field("modified"), order.Descending)

	b.Reset()
	b.WriteString("SELECT DISTINCT c.id, c.title, c.modified")
	b.WriteString("\nFROM ")
	comics.Eval(&b)
	o.Eval(&b)
	b.WriteString(fmt.Sprintf("\nLIMIT %s", parameters.New(limit)))
	b.WriteString(fmt.Sprintf("\nOFFSET %s", parameters.New(offset)))

	return c, db.QueryRows(
		db.Context,
		b.String(),
		parameters...,
	)(func(scan db.Scanner) error {
		var (
			comic = new(Comic)
			trash string
		)

		err = scan(&comic.ID, &comic.Title, &trash)
		if err != nil {
			return err
		}

		err = comic.fetch()
		if err != nil {
			return err
		}

		c.Comics = append(c.Comics, comic)

		return nil
	})
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
	summary, ok := comicTagSummaryCache.Get(comic.ID)
	if ok {
		comic.TagSummary = summary
		return nil
	}

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

	comicTagSummaryCache.Set(comic.ID, comic.TagSummary)
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
