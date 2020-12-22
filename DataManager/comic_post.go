package DataManager

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/querier"
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
	Order   int // TODO: Major, Minor pages
	Chapter *Chapter
	Comic   *Comic
}

func (p *ComicPost) QID(q querier.Q) int {
	if p.ID != 0 {
		return p.ID
	}

	err := q.QueryRow("SELECT id FROM comic_mappings WHERE chapter_id = $1 AND post_id=$2", p.Chapter.QID(q), p.Post.ID).Scan(&p.ID)
	if err != nil {
		log.Print(err)
	}
	return p.ID
}

func (p *ComicPost) QChapter(q querier.Q) error {
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

//func (p *ComicPost) QComic(q querier.Q) error {
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

func (p *ComicPost) QPost(q querier.Q) error {
	if p.Post != nil {
		return nil
	}

	var post = NewPost()

	if err := q.QueryRow("SELECT post_id FROM comic_mappings WHERE id = $1", p.ID).Scan(&post.ID); err != nil {
		log.Println(err)
		return err
	}

	p.Post = post

	return nil
}

func (p *ComicPost) QOrder(q querier.Q) int {
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

	dupe, err := getDupeFromPost(tx, p.Post)
	if err != nil {
		return err
	}
	p.Post = dupe.Post

	if p.Post.ID == 0 {
		return fmt.Errorf("Invalid post")
	}

	err = tx.QueryRow("INSERT INTO comic_mappings(post_id, post_order, Chapter_id) Values($1, $2, $3) RETURNING id", p.Post.ID, p.Order, p.Chapter.QID(tx)).Scan(&p.ID)
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

	C.Cache.Purge("CCH", strconv.Itoa(p.Chapter.QID(tx)))

	err = p.Chapter.QComic(tx)
	if err != nil {
		log.Println(err)
		return err
	}

	C.Cache.Purge(cacheComic, strconv.Itoa(p.Chapter.Comic.ID))

	return nil
}

func (p *ComicPost) SaveEdit(user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return err
	}

	defer tx.Commit()

	if !user.QFlag(tx).Comics() {
		return fmt.Errorf("Action not allowed from this user")
	}

	_, err = tx.Exec(
		"UPDATE comic_mappings SET post_order = $1, post_id = $2, Chapter_id = $3 WHERE id = $4",
		p.Order,
		p.Post.ID,
		p.Chapter.ID,
		p.ID,
	)
	if err != nil {
		log.Print(err)
		tx.Rollback()
		return err
	}

	if err = p.log(tx, lUpdate, user); err != nil {
		log.Println(err)
		tx.Rollback()
		return err
	}

	return nil
}

func (p *ComicPost) replacePost(q querier.Q, new *Post) error {
	if new.ID == 0 {
		return errors.New("new.id is zero")
	}
	if p.Post.ID == 0 {
		return errors.New("p.ID is zero")
	}
	_, err := q.Exec("UPDATE comic_mappings SET post_id=$1 WHERE post_id=$2", new.ID, p.Post.ID)
	if err != nil {
		log.Print(err)
		return err
	}

	return nil
}

func (p *ComicPost) Delete(user *User) error {
	tx, err := DB.Begin()
	if err != nil {
		log.Println(err)
		return err
	}
	var def func() error
	def = tx.Rollback
	defer func() {
		def()
		return
	}()

	if err = p.QPost(tx); err != nil {
		return err
	}
	if err = p.QChapter(tx); err != nil {
		return err
	}
	p.QOrder(tx)

	_, err = tx.Exec("DELETE FROM comic_mappings WHERE id = $1", p.ID)
	if err != nil {
		log.Println(err)
		return err
	}

	if err = p.log(tx, lRemove, user); err != nil {
		log.Println(err)
		return err
	}

	err = p.Chapter.QComic(tx)
	if err != nil {
		log.Println(err)
		return err
	}

	C.Cache.Purge(cacheComic, strconv.Itoa(p.Chapter.Comic.ID))

	def = tx.Commit

	return nil
}
