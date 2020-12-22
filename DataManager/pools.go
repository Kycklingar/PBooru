package DataManager

import (
	"errors"
	"log"

	"github.com/kycklingar/PBooru/DataManager/querier"
)

type Pool struct {
	ID          int
	User        *User
	Title       string
	Description string

	Posts []PoolMapping
}

func NewPool() *Pool {
	var p Pool
	p.User = NewUser()

	return &p
}

type PoolMapping struct {
	Post     *Post
	Position int
}

func (p *Pool) QTitle(q querier.Q) string {
	if len(p.Title) > 0 {
		return p.Title
	}

	if p.ID == 0 {
		return ""
	}

	err := q.QueryRow("SELECT title FROM user_pools WHERE id = $1", p.ID).Scan(&p.Title)
	if err != nil {
		log.Println(err)
		return ""
	}

	return p.Title
}

func (p *Pool) QUser(q querier.Q) *User {
	if p.User.QID(q) != 0 {
		return p.User
	}

	if p.ID == 0 {
		return p.User
	}

	err := q.QueryRow("SELECT user_id FROM user_pools WHERE id = $1", p.ID).Scan(&p.User.ID)
	if err != nil {
		log.Println(err)
	}

	return p.User
}

func (p *Pool) QDescription(q querier.Q) string {
	if len(p.Description) > 0 {
		return p.Description
	}

	if p.ID == 0 {
		return ""
	}

	err := q.QueryRow("SELECT description FROM user_pools WHERE id = $1", p.ID).Scan(&p.Description)
	if err != nil {
		log.Println(err)
		return ""
	}

	return p.Description
}

func (p *Pool) QPosts(q querier.Q) error {
	if len(p.Posts) > 0 {
		return nil
	}
	rows, err := q.Query("SELECT post_id, position FROM pool_mappings WHERE pool_id = $1 ORDER BY (position, post_id) DESC", p.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pm PoolMapping
		pm.Post = NewPost()
		err = rows.Scan(&pm.Post.ID, &pm.Position)
		if err != nil {
			log.Println(err)
			return err
		}

		p.Posts = append(p.Posts, pm)
	}

	return rows.Err()
}

func (p *Pool) PostsLimit(limit int) []PoolMapping {
	return p.Posts[:Smal(limit, len(p.Posts))]
}

func (p *Pool) Save(q querier.Q) error {
	if len(p.Title) <= 0 {
		return errors.New("Title cannot be empty")
	}

	if p.User.QID(q) == 0 {
		return errors.New("No user in pool")
	}

	_, err := q.Exec("INSERT INTO user_pools(user_id, title, description) VALUES($1, $2, $3)", p.User.QID(q), p.Title, p.Description)
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (p *Pool) Add(postID int) error {
	if p.ID <= 0 {
		return errors.New("Pool id is 0")
	}

	if postID <= 0 {
		return errors.New("Post id is < 0")
	}

	_, err := DB.Exec("INSERT INTO pool_mappings(pool_id, post_id, position) VALUES($1, $2, COALESCE((SELECT MAX(position) + 1 FROM pool_mappings WHERE pool_id = $3), 0))", p.ID, postID, p.ID)
	return err
}

func (p *Pool) RemovePost(postID int) error {
	if p.ID <= 0 {
		return errors.New("poolID is <= 0")
	}

	_, err := DB.Exec("DELETE FROM pool_mappings WHERE pool_id = $1 AND post_id = $2", p.ID, postID)
	return err
}
