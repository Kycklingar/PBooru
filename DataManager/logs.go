package DataManager

import (
	"fmt"
	"time"

	"github.com/kycklingar/PBooru/DataManager/timestamp"
	"github.com/kycklingar/sqhell/cond"
)

var logTableGetFuncs = make(map[logtable]logTableGetFunc)

type logTableGetFunc func(*Log, querier) error

type Log struct {
	ID        int
	User      *User
	Timestamp timestamp.Timestamp

	// Post logs
	Posts postHistoryMap

	// Alts
	Alts []logAlts

	// Alias
	Aliases logAliasMap

	// Parents
	Parents logParent

	// Multi tags
	MultiTags map[lAction][]logMultiTags

	// Comics
	Comic      logComic
	Chapters   []logChapter
	ComicPages []logComicPage
}

type LogCategory int

const (
	LogNoCat LogCategory = iota
	LogCatPost
	LogCatComic
	LogCatChapter
	LogCatComicPage
)

type LogSearchOptions struct {
	Category LogCategory
	CatVal   int

	UserID int

	DateSince time.Time
	DateUntil time.Time

	Limit  int
	Offset int
}

func SearchLogs(opts LogSearchOptions) ([]Log, int, error) {
	var (
		join  = new(cond.Group)
		where = new(cond.Group)
		limit = new(cond.Group).
			Add("", cond.P("Limit $%d")).
			Add("\n", cond.P("Offset $%d"))
		v []any
		w string

		paramIndex = 1
		count      int
		o          int
	)

	switch opts.Category {
	case LogCatPost:
		join.Add("\n",
			cond.N(`
			JOIN log_post p
			ON l.log_id = p.log_id
			`),
		)
		where.Add("\nAND", cond.P(`p.post_id = $%d`))
		v = append(v, opts.CatVal)
	case LogCatComic:
		join.Add("\n",
			cond.N(`
			LEFT JOIN log_comics lc
			ON l.log_id = lc.log_id
			LEFT JOIN log_chapters lcc
			ON l.log_id = lcc.log_id
			`,
			),
		)
		where.Add(
			"\nAND",
			new(cond.Group).
				Add("", cond.O{S: "lc.id = $%d", I: &o}).
				Add(" OR", cond.O{S: "lcc.comic_id = $%d", I: &o}),
		)
		v = append(v, opts.CatVal)
	case LogCatChapter:
		join.Add("\n",
			cond.N(`
			LEFT JOIN log_chapters lcc
			ON l.log_id = lcc.log_id
			LEFT JOIN log_comic_page lcp
			ON l.log_id = lcp.log_id
			`,
			),
		)
		where.Add("\nAND",
			new(cond.Group).
				Add("", cond.O{S: "lcc.chapter_id = $%d", I: &o}).
				Add(" OR", cond.O{S: "lcp.chapter_id = $%d", I: &o}),
		)
		v = append(v, opts.CatVal)
	case LogCatComicPage:
		join.Add("\n",
			cond.N(`
			LEFT JOIN log_comic_page lcp
			ON l.log_id = lcp.log_id
			`,
			),
		)
		where.Add("\nAND",
			cond.P("lcp.comic_page_id = $%d"),
		)
		v = append(v, opts.CatVal)
	}

	if opts.UserID != 0 {
		where.Add("\nAND",
			cond.P("l.user_id = $%d"),
		)
		v = append(v, opts.UserID)
	}

	if !opts.DateSince.IsZero() {
		where.Add("\nAND",
			cond.P("l.timestamp > $%d"),
		)
		v = append(v, opts.DateSince)
	}

	if !opts.DateUntil.IsZero() {
		where.Add("\nAND",
			cond.P("l.timestamp < $%d"),
		)
		v = append(v, opts.DateUntil)
	}

	if len(v) > 0 {
		w = "WHERE " + where.Eval(&paramIndex)
	}

	queryCount := fmt.Sprintf(`
		SELECT count(DISTINCT l.log_id)
		FROM logs l
		%s
		%s
		`,
		join.Eval(&paramIndex),
		w,
	)
	err := DB.QueryRow(queryCount, v...).Scan(&count)
	if err != nil {
		return nil, count, err
	}

	paramIndex = 1

	if len(v) > 0 {
		o = 0
		w = "WHERE " + where.Eval(&paramIndex)
	}
	v = append(v, opts.Limit, opts.Offset)

	query := fmt.Sprintf(`
		SELECT DISTINCT l.log_id, l.user_id, l.timestamp
		FROM logs l
		%s
		%s
		ORDER BY l.timestamp DESC, l.log_id DESC
		%s
		`,
		join.Eval(&paramIndex),
		w,
		limit.Eval(&paramIndex),
	)

	logs, err := logs(DB, query, v...)
	return logs, count, err
}

func logs(q querier, query string, values ...interface{}) ([]Log, error) {
	var logs []Log

	err := func() error {
		rows, err := q.Query(query, values...)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l = Log{
				User:  NewUser(),
				Posts: make(postHistoryMap),
			}

			if err = rows.Scan(&l.ID, &l.User.ID, &l.Timestamp); err != nil {
				return err
			}

			logs = append(logs, l)
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	for i := range logs {
		if err = logs[i].altered(q); err != nil {
			return nil, err
		}
	}

	return logs, nil
}

func (l *Log) altered(q querier) error {
	tables, err := func() ([]logtable, error) {
		rows, err := q.Query(`
			SELECT table_name
			FROM logs_tables lt
			JOIN logs_tables_altered lta
			ON lt.id = lta.table_id
			WHERE log_id = $1
			`,
			l.ID,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var tables []logtable

		for rows.Next() {
			var t logtable

			if err = rows.Scan(&t); err != nil {
				return nil, err
			}

			tables = append(tables, t)
		}
		return tables, nil
	}()
	if err != nil {
		return err
	}

	for _, table := range tables {
		if err = logTableGetFuncs[table](l, q); err != nil {
			return err
		}
	}

	return nil
}
