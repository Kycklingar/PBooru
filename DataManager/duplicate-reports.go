package DataManager

import (
	"database/sql"
	"fmt"
	"log"
)

type DupReport struct {
	ID         int
	ReportType reportType
	Reporter   *User
	Note       string
	Approved   timestamp
	Timestamp  timestamp
	Dupe       Dupe
}

func FetchDupReports(limit, offset int, asc, approved, pluckedReports bool) ([]*DupReport, error) {
	var (
		reports []*DupReport
		order   = "DESC"
		pluck   = "AND report_type = 0"
		apprvd  = "NULL"
		ocolumn = "timestamp"
	)

	if asc {
		order = "ASC"
	}

	if pluckedReports {
		pluck = "AND report_type = 1"
	}

	if approved {
		apprvd = "NOT NULL"
		ocolumn = "approved"
	}

	err := func() error {
		rows, err := DB.Query(
			fmt.Sprintf(`
				SELECT id, report_type, post_id, reporter, note, approved, timestamp
				FROM duplicate_report
				WHERE approved IS %s
				%s
				ORDER BY %s %s
				LIMIT $1
				OFFSET $2
				`,
				apprvd,
				pluck,
				ocolumn, order,
			),
			limit,
			offset,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var dr = new(DupReport)
			dr.Reporter = NewUser()
			dr.Dupe.Post = NewPost()
			err = rows.Scan(&dr.ID, &dr.ReportType, &dr.Dupe.Post.ID, &dr.Reporter.ID, &dr.Note, &dr.Approved, &dr.Timestamp)
			if err != nil {
				return err
			}

			reports = append(reports, dr)
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	for _, report := range reports {
		err = report.QInferior(DB)
		if err != nil {
			return nil, err
		}
	}

	return reports, nil
}

func FetchDupReport(id int, q querier) (*DupReport, error) {
	var r DupReport
	r.ID = id
	r.Reporter = NewUser()
	r.Dupe.Post = NewPost()

	err := q.QueryRow(`
		SELECT post_id, report_type, reporter, note, approved, timestamp
		FROM duplicate_report
		WHERE id = $1
		`, id).Scan(&r.Dupe.Post.ID, &r.ReportType, &r.Reporter.ID, &r.Note, &r.Approved, &r.Timestamp)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	query := `
		SELECT post_id
		FROM duplicate_report_posts
		WHERE report_id = $1
		`

	rows, err := q.Query(query, id)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var p = NewPost()

		err = rows.Scan(&p.ID)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		r.Dupe.Inferior = append(r.Dupe.Inferior, p)
	}

	return &r, rows.Err()
}

func ProcessDupReport(reportID int) error {
	_, err := DB.Exec(`
		UPDATE duplicate_report
		SET approved = CURRENT_TIMESTAMP
		WHERE id = $1
		`,
		reportID,
	)

	return err
}

type reportType int

const (
	RDupe reportType = iota
	RNonDupe
)

func ReportDuplicates(dupe Dupe, reporter *User, note string, repT reportType) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	// Make sure they are part of the apple-tree
	// Only if reportType = RNonDupe
	if repT == RNonDupe {
		dupe, err = func() (Dupe, error) {
			var (
				d      = Dupe{Post: NewPost()}
				infids []int
			)

			for _, p := range dupe.Inferior {
				infids = append(infids, p.ID)
			}

			rows, err := tx.Query(
				fmt.Sprintf(`
			SELECT apple, pear
			FROM apple_tree
			WHERE (
				apple = $1
				AND pear IN(%s)
			) AND processed IS NULL
			UNION ALL
			SELECT pear, apple
			FROM apple_tree
			WHERE (
				apple IN(%s)
				AND pear = $1
			) AND processed IS NULL
			`,
					strSep(infids, ","),
					strSep(infids, ","),
				),
				dupe.Post.ID,
			)
			if err != nil {
				return d, err
			}
			defer rows.Close()

			for rows.Next() {
				var p = NewPost()
				if err = rows.Scan(&d.Post.ID, &p.ID); err != nil {
					return d, err
				}

				d.Inferior = append(d.Inferior, p)
			}

			return d, nil
		}()
		if err != nil {
			return err
		}
	}

	if len(dupe.Inferior) <= 0 {
		return nil
	}

	var reportID int

	if err = tx.QueryRow(`
		INSERT INTO duplicate_report (
			post_id,
			reporter,
			note,
			report_type
		)
		VALUES(
			$1,
			$2,
			$3,
			$4
		)
		RETURNING id`,
		dupe.Post.ID,
		reporter.ID,
		note,
		repT,
	).Scan(&reportID); err != nil {
		return err
	}

	query := `
	INSERT INTO duplicate_report_posts (
		report_id,
		post_id
	)
	VALUES(
		$1,
		$2
	)`

	for _, p := range dupe.Inferior {
		if _, err = tx.Exec(
			query,
			reportID,
			p.ID,
		); err != nil {
			return err
		}
	}

	return nil

}

func (r *DupReport) QInferior(q querier) error {
	rows, err := q.Query(`
		SELECT post_id
		FROM duplicate_report_posts
		WHERE report_id = $1
		`,
		r.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p = NewPost()
		err = rows.Scan(&p.ID)
		if err != nil {
			return err
		}

		r.Dupe.Inferior = append(r.Dupe.Inferior, p)
	}

	return rows.Err()
}

// Cleanup non apple-tree reports
func DuplicateReportCleanup() (int64, error) {
	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	defer commitOrDie(tx, &err)

	var res sql.Result

	_, err = tx.Exec(`
		WITH reports AS (
			SELECT dr.id, dr.post_id AS lr, drp.post_id AS rr, report_type
			FROM duplicate_report dr
			LEFT JOIN duplicate_report_posts drp
			ON dr.id = drp.report_id
			WHERE approved IS NULL
		)

		DELETE FROM duplicate_report_posts
		WHERE report_id IN(
			SELECT reports.id
			FROM apple_tree
			RIGHT JOIN reports
			ON (
				(
					reports.lr = apple
					AND reports.rr = pear
				) OR (
					reports.lr = pear
					AND reports.rr = apple
				)
			)
			WHERE reports.report_type = 1
			AND apple IS NULL
		)
		`,
	)
	if err != nil {
		return 0, err
	}

	res, err = tx.Exec(`
		DELETE FROM duplicate_report
		WHERE id IN(
			SELECT dr.id
			FROM duplicate_report dr
			LEFT JOIN duplicate_report_posts drp
			ON dr.id = drp.report_id
			WHERE drp.post_id IS NULL
		)
		`,
	)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()

}
