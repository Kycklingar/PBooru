package DataManager

import (
	"log"
)

type DupReport struct {
	ID        int
	Reporter  *User
	Note      string
	Approved  timestamp
	Timestamp timestamp
	Dupe      Dupe
}

func FetchDupReports(limit, offset int) ([]*DupReport, error) {
	var reports []*DupReport
	err := func() error {
		rows, err := DB.Query(`
			SELECT id, post_id, reporter, note, approved, timestamp
			FROM duplicate_report
			ORDER BY approved DESC NULLS FIRST
			LIMIT $1
			OFFSET $2
			`,
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
			err = rows.Scan(&dr.ID, &dr.Dupe.Post.ID, &dr.Reporter.ID, &dr.Note, &dr.Approved, &dr.Timestamp)
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
		SELECT post_id, reporter, note, approved, timestamp
		FROM duplicate_report
		WHERE id = $1
		`, id).Scan(&r.Dupe.Post.ID, &r.Reporter.ID, &r.Note, &r.Approved, &r.Timestamp)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	rows, err := q.Query("SELECT post_id FROM duplicate_report_posts WHERE report_id = $1", id)
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

func ReportDuplicates(dupe Dupe, reporter *User, note string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer commitOrDie(tx, &err)

	var reportID int

	if err = tx.QueryRow(`
		INSERT INTO duplicate_report (
			post_id,
			reporter,
			note
		)
		VALUES(
			$1,
			$2,
			$3
		)
		RETURNING id`,
		dupe.Post.ID,
		reporter.ID,
		note,
	).Scan(&reportID); err != nil {
		return err
	}

	for _, p := range dupe.Inferior {
		if _, err = tx.Exec(`
			INSERT INTO duplicate_report_posts (
				report_id,
				post_id
			)
			VALUES(
				$1,
				$2
			)`,
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
