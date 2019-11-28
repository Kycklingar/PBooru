package DataManager

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
)

func newComicPost() *ComicPost {
	var cp ComicPost
	cp.Chapter = newChapter()
	cp.Post = NewPost()
	cp.Comic = NewComic()
	cp.Order = 0
	return &cp
}
func NewComicPost() *ComicPost {
	var cp ComicPost
	cp.Order = 0
	return &cp
}

type ComicPost struct {
	ID      int
	Post    *Post
	Order   int
	Chapter *Chapter
	Comic   *Comic
}

func (p *ComicPost) QID(q querier) int {
	if p.ID != 0 {
		return p.ID
	}

	err := q.QueryRow("SELECT id FROM comic_mappings WHERE chapter_id = $1 AND post_id=$2", p.Chapter.QID(q), p.Post.QID(q)).Scan(&p.ID)
	if err != nil {
		log.Print(err)
	}
	return p.ID
}

func (p *ComicPost) QChapter(q querier) error {
	if p.Chapter != nil {
		return nil
	}

	var chapter Chapter
	if err := q.QueryRow("SELECT chapter_id FROM comic_mappings WHERE id = $1", p.ID).Scan(&chapter.ID); err != nil {
		log.Println(err)
		return err
	}

	p.Chapter = &chapter

	return nil
}

//func (p *ComicPost) QComic(q querier) error {
//	if p.Comic != nil {
//		return nil
//	}
//
//	comic := NewComic()
//	if err := q.QueryRow("SELECT comic_id FROM comic_mappings WHERE id = $1", p.ID).Scan(&comic.ID); err != nil {
//		log.Println(err)
//		return err
//	}
//
//	p.Comic = comic
//
//	return nil
//}

func (p *ComicPost) QPost(q querier) error {
	if p.Post != nil {
		return nil
	}

	var post = NewPost()

	if err := q.QueryRow("SELECT post_id FROM comic_mappings WHERE id = $1", p.ID).Scan(&post.ID); err != nil {
		log.Println(err)
		return nil
	}

	p.Post = post

	return nil
}

func (p *ComicPost) QOrder(q querier) int {
	if p.Order >= 0 {
		return p.Order
	}
	if err := q.QueryRow("SELECT post_order FROM comic_mappings WHERE id = $1", p.ID).Scan(&p.Order); err != nil {
		log.Println(err)
	}
	return p.Order
}

func (p *ComicPost) Save(user *User, overwrite bool) error {
	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return err
	}

	defer tx.Commit()

	if !user.QFlag(tx).Comics() {
		return fmt.Errorf("Action not allowed from this user")
	}

	if p.Chapter.QID(tx) == 0 {
		return fmt.Errorf("Invalid Chapter")
	}

	if overwrite && p.QID(tx) != 0 {
		_, err := tx.Exec("UPDATE comic_mappings SET post_order=$1, Chapter_id=$2 WHERE id = $3", p.Order, p.Chapter.QID(tx), p.ID)
		if err != nil {
			log.Print(err)
			tx.Rollback()
			return err
		}

		p.Comic.QID(tx)
		p.QChapter(tx)
		p.Chapter.QID(tx)
		p.QPost(tx)
		p.Post.QID(tx)

		if err = p.log(tx, lUpdate, user); err != nil {
			log.Println(err)
			tx.Rollback()
			return err
		}

	} else {
		if p.Post.QID(tx) == 0 {
			return fmt.Errorf("Invalid post")
		}

		if p.Comic.QID(tx) == 0 {
			return fmt.Errorf("Invalid comic")
		}

		err := tx.QueryRow("INSERT INTO comic_mappings(post_id, post_order, Chapter_id) Values($1, $2, $3) RETURNING id", p.Post.QID(tx), p.Order, p.Chapter.QID(tx)).Scan(&p.ID)
		if err != nil {
			log.Print(err)
			tx.Rollback()
			return err
		}

		if err = p.log(tx, lCreate, user); err != nil {
			log.Println(err)
			tx.Rollback()
			return err
		}
	}

	C.Cache.Purge("CCH", strconv.Itoa(p.Chapter.QID(tx)))

	return nil
}

func (p *ComicPost) replacePost(q querier, new *Post) error {
	if new.QID(q) == 0 {
		return errors.New("new.id is zero")
	}
	if p.Post.QID(q) == 0 {
		return errors.New("p.ID is zero")
	}
	_, err := q.Exec("UPDATE comic_mappings SET post_id=$1 WHERE post_id=$2", new.QID(q), p.Post.QID(q))
	if err != nil {
		log.Print(err)
		return err
	}

	return nil
}
