package DataManager

import (
	"fmt"
	"log"
)

type DupReport struct {
	ID        int
	ReportType reportType
	Reporter  *User
	Note      string
	Approved  timestamp
	Timestamp timestamp
	Dupe      Dupe
}

func FetchDupReports(limit, offset int, asc bool) ([]*DupReport, error) {
	var reports []*DupReport

	var order = "DESC"

	if asc {
		order = "ASC"
	}

	err := func() error {
		rows, err := DB.Query(
			fmt.Sprintf(`
				SELECT id, report_type, post_id, reporter, note, approved, timestamp
				FROM duplicate_report
				WHERE approved IS NULL
				ORDER BY timestamp %s
				LIMIT $1
				OFFSET $2
				`,
				order,
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
