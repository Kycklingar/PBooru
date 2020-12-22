package DataManager

import (
	"database/sql"
	"errors"
	"log"
	"strconv"

	C "github.com/kycklingar/PBooru/DataManager/cache"
	"github.com/kycklingar/PBooru/DataManager/querier"
	ts "github.com/kycklingar/PBooru/DataManager/timestamp"
	"golang.org/x/crypto/bcrypt"
)

var (
	WrongPassword = errors.New("Wrong Password")
)

func NewUser() *User {
	var u User
	//u.Session = &Session{}
	u.Messages = &messages{}
	u.Messages.All = nil
	u.Messages.Sent = nil
	u.Messages.Unread = nil

	return &u
}

func CachedUser(u *User) *User {
	if n := C.Cache.Get("USR", strconv.Itoa(u.ID)); n != nil {
		cu, ok := n.(*User)
		if !ok {
			log.Println("not a *User in cache")
			return u
		}
		return cu
	} else {
		C.Cache.Set("USR", strconv.Itoa(u.ID), u)
	}
	return u
}

func Login(username, password string) (string, error) {
	var (
		hash, salt string
		userID     int
	)

	err := DB.QueryRow(`
		SELECT hash, salt, users.id
		FROM passwords
		JOIN users
		ON user_id = users.id
		WHERE username = $1
		`,
		username,
	).Scan(&hash, &salt, &userID)
	if err != nil {
		return "", err
	}

	if err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password+salt)); err != nil {
		return "", WrongPassword
	}

	return sessionNew(DB, userID)
}

func Logout(key string) error {
	return sessionDestroy(DB, key)
}

func Sessioned(key string) *User {
	var (
		u   = NewUser()
		err error
	)

	u.ID, err = sessionGet(DB, key)
	if err != nil {
		//log.Println(err)
		return u
	}

	return CachedUser(u)
}

func Register(username, password string) error {
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer commitOrDie(tx, &err)

	err = register(tx, username, password, flag(CFG.StdUserFlag))

	return err
}

func register(q querier.Q, username, password string, admFlag flag) error {
	var (
		id    int
		salt  string = randomString(64)
		bhash []byte
		hash  string
		err   error
	)

	bhash, err = bcrypt.GenerateFromPassword([]byte(password+salt), 0)
	if err != nil {
		return err
	}

	hash = string(bhash)

	if err = q.QueryRow(`
		SELECT id
		FROM users
		WHERE LOWER(username) = LOWER($1)
		`,
		username,
	).Scan(&id); err != nil && err != sql.ErrNoRows {
		return err
	}

	if id != 0 {
		return errors.New("Username already exist")
	}

	if err = q.QueryRow(`
		INSERT INTO users(username, datejoined, adminflag)
		VALUES($1, CURRENT_TIMESTAMP, $2)
		RETURNING id
		`,
		username,
		admFlag,
	).Scan(&id); err != nil {
		return err
	}

	_, err = q.Exec(`
		INSERT INTO passwords(user_id, hash, salt)
		VALUES($1, $2, $3)
		`,
		id,
		hash,
		salt,
	)

	return err
}

type User struct {
	ID           int
	Name         string
	passwordHash string
	salt         string
	Joined       ts.Timestamp
	flag         *flag

	Messages *messages

	editCount *int

	Pools []*Pool
}

func (u *User) QID(q querier.Q) int {
	if u.ID != 0 {
		return u.ID
	}
	if u.Name != "" {
		err := q.QueryRow("SELECT id FROM users WHERE username=$1", u.Name).Scan(&u.ID)
		if err != nil {
			log.Print(err)
		}
	}

	return u.ID
}

func (u *User) Title() string {
	ec := u.tagEditCount(DB)
	var title string
	switch {
	case ec > 100000:
		title = "Archivist"
	case ec > 10000:
		title = "Godlike"
	case ec > 1000:
		title = "Respectable"
	case ec > 100:
		title = "Tagger"
	case ec > 10:
		title = "Contributor"
	}

	return title
}

func (u *User) tagEditCount(q querier.Q) int {
	if u.editCount != nil {
		return *u.editCount
	}
	u.editCount = new(int)
	err := q.QueryRow("SELECT count(*) FROM tag_history th JOIN edited_tags et ON th.id = et.history_id WHERE th.user_id = $1", u.QID(q)).Scan(u.editCount)
	if err != nil {
		log.Println(err)
		return 0
	}

	return *u.editCount
}

func (u *User) SetID(q querier.Q, id int) error {
	if u.ID > 0 {
		return nil
	}
	if err := q.QueryRow("SELECT id FROM users WHERE id = $1", id).Scan(&u.ID); err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func (u *User) RecentPosts(q querier.Q, limit int) []*Post {
	rows, err := q.Query("SELECT id FROM posts WHERE uploader = $1 ORDER BY id DESC LIMIT $2", u.QID(q), limit)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()

	var posts []*Post

	for rows.Next() {
		var p = NewPost()
		err = rows.Scan(&p.ID)
		if err != nil {
			log.Println(err)
			return nil
		}
		posts = append(posts, p)
	}

	if err = rows.Err(); err != nil {
		log.Println(err)
		return nil
	}

	return posts
}

func (u *User) QName(q querier.Q) string {
	if u.Name != "" {
		return u.Name
	}
	if u.QID(q) == 0 {
		return ""
	}
	err := q.QueryRow("SELECT username FROM users WHERE id=$1", u.QID(q)).Scan(&u.Name)
	if err != nil {
		log.Print(err)
	}
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}

func (u *User) Flag() flag {
	if u.flag != nil {
		return *u.flag
	}
	return flag(0)
}

func (u *User) SetFlag(q querier.Q, f int) error {
	if u.ID <= 0 {
		return errors.New("No user to set flag on")
	}

	_, err := q.Exec("UPDATE users SET adminflag = $1 WHERE id = $2", f, u.ID)
	if err != nil {
		log.Println(err)
		return err
	}

	if u.flag == nil {
		u.flag = new(flag)
	}

	*u.flag = flag(f)

	return nil
}

func (u *User) QFlag(q querier.Q) flag {
	if u.flag != nil {
		return *u.flag
	}
	if u.QID(q) == 0 {
		return flag(0)
	}

	var f flag
	u.flag = &f
	err := q.QueryRow("SELECT adminflag FROM users WHERE id=$1", u.QID(q)).Scan(&u.flag)
	if err != nil {
		log.Print(err)
	}
	return *u.flag
}

func (u *User) Voted(q querier.Q, p *Post) bool {
	if p.ID <= 0 {
		return false
	}

	if u.QID(q) <= 0 {
		return false
	}

	var v int

	if err := q.QueryRow("SELECT count(*) FROM post_score_mapping WHERE post_id = $1 AND user_id = $2", p.ID, u.ID).Scan(&v); err != nil {
		log.Println(err)
		return false
	}
	return v > 0
}

func (u *User) QPools(q querier.Q) []*Pool {
	if u.Pools != nil {
		return u.Pools
	}

	rows, err := q.Query("SELECT id, title, description FROM user_pools WHERE user_id = $1", u.QID(q))
	if err != nil {
		log.Println(err)
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var p Pool
		err = rows.Scan(&p.ID, &p.Title, &p.Description)
		if err != nil {
			log.Println(err)
			return nil
		}

		p.User = u

		u.Pools = append(u.Pools, &p)
	}

	return u.Pools
}

func (u *User) RecentVotes(q querier.Q, limit int) (posts []*Post) {
	rows, err := q.Query("SELECT post_id FROM post_score_mapping WHERE user_id = $1 ORDER BY id DESC LIMIT $2", u.QID(DB), limit)
	if err != nil {
		log.Println(err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var p = NewPost()
		if err = rows.Scan(&p.ID); err != nil {
			log.Println(err)
			return
		}
		posts = append(posts, p)
	}
	if err = rows.Err(); err != nil {
		log.Println(err)
	}

	return
}

func (u *User) Sessions(q querier.Q) []*Session {
	rows, err := q.Query("SELECT sesskey, expire FROM sessions WHERE user_id=$1", u.QID(q))
	if err != nil {
		log.Println(err)
		return nil
	}

	var sessions []*Session

	for rows.Next() {
		s := &Session{User: u.ID}
		if err = rows.Scan(&s.Key, &s.Accessed); err != nil {
			log.Println(err)
			return nil
		}

		sessions = append(sessions, s)
	}

	return sessions
}

