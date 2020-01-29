package DataManager

import (
	"database/sql"
	"log"
	"time"
)

type DupReport struct {
	ID        int
	Reporter  *User
	Note      string
	Approved  int
	Timestamp time.Time
	Posts     []DRPost
}

type DRPost struct {
	Post  *Post
	Score int
}

func FetchDupReports(limit, offset int) ([]*DupReport, error) {
	var reports []*DupReport
	err := func () error {
		rows, err := DB.Query(`
			SELECT id, reporter, note, approved, timestamp
			FROM duplicate_report
			LIMIT $1
			OFFSET $2
			`,
			limit,
			offset,
		)
		defer rows.Close()

		var timestamp string

		for rows.Next() {
			var dr = new(DupReport)
			dr.Reporter = NewUser()
			err = rows.Scan(&dr.ID, &dr.Reporter.ID, &dr.Note, &dr.Approved, &timestamp)
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
		err = report.QDRPosts(DB)
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

	var timestamp string

	err := q.QueryRow(`
		SELECT reporter, note, approved, timestamp
		FROM duplicate_report
		WHERE id = $1
		`, id).Scan(&r.Reporter.ID, &r.Note, &r.Approved, &timestamp)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	rows, err := q.Query("SELECT post_id, score FROM duplicate_report_posts WHERE report_id = $1", id)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var dp DRPost
		dp.Post = NewPost()
		err = rows.Scan(&dp.Post.ID, &dp.Score)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		r.Posts = append(r.Posts, dp)
	}

	return &r, rows.Err()
}

func ReportDuplicates(posts []DRPost, reporter *User, note string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}

	defer func(tx *sql.Tx, err error) {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}(tx, err)

	var reportID int

	if err = tx.QueryRow(`
		INSERT INTO duplicate_report(
			reporter,
			note
		)
		VALUES(
			$1,
			$2
		)
		RETURNING id`,
		reporter.ID,
		note,
	).Scan(&reportID); err != nil {
		return err
	}

	for _, pid := range posts {
		if _, err = tx.Exec(`
			INSERT INTO duplicate_report_posts (
				report_id,
				post_id,
				score
			)
			VALUES(
				$1,
				$2,
				$3
			)`,
			reportID,
			pid.Post.ID,
			pid.Score,
		); err != nil {
			return err
		}
	}

	return nil

}

func (r *DupReport) QDRPosts(q querier) error {
	rows, err := q.Query(`
		SELECT post_id, score
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
		var d DRPost
		d.Post = NewPost()
		err = rows.Scan(&d.Post.ID, &d.Score)
		if err != nil {
			return err
		}

		r.Posts = append(r.Posts, d)
	}

	return rows.Err()
}
