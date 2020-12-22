package DataManager

import (
	"errors"
	"log"

	"github.com/kycklingar/PBooru/DataManager/querier"
)

func NewReport() Report {
	return Report{Post: NewPost(), Reporter: NewUser()}
}

type Report struct {
	ID          int
	Post        *Post
	Reporter    *User
	Reason      int
	Description string
}

func GetReports(q querier.Q) ([]*Report, error) {
	rows, err := q.Query("SELECT id, post_id, reporter, reason, description FROM reports ORDER BY id DESC LIMIT 25")
	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer rows.Close()

	var reports []*Report

	for rows.Next() {
		var r = NewReport()
		err = rows.Scan(&r.ID, &r.Post.ID, &r.Reporter.ID, &r.Reason, &r.Description)
		if err != nil {
			log.Println(err)
			return nil, err
		}

		reports = append(reports, &r)
	}

	return reports, nil
}

func (r Report) Delete(q querier.Q) error {
	if r.ID <= 0 {
		return errors.New("report.ID <= 0")
	}

	_, err := q.Exec("DELETE FROM reports WHERE id = $1", r.ID)
	return err
}

func (r Report) Submit() error {
	if r.Post.ID <= 0 {
		return errors.New("no Post.ID in report")
	}

	if r.Reporter.ID <= 0 {
		return errors.New("no Reporter.ID in report")
	}

	_, err := DB.Exec("INSERT INTO reports(post_id, reporter, reason, description) VALUES($1, $2, $3, $4)", r.Post.ID, r.Reporter.ID, r.Reason, r.Description)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}
