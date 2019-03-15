package DataManager

import (
	"container/list"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	mrand "math/rand"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func NewUser() *User {
	var u User
	u.Session = &Session{}
	return &u
}

type User struct {
	ID           int
	Name         string
	passwordHash string
	salt         string
	Joined       time.Time
	flag         *flag
	Session      *Session

	Pools []*Pool
}

type flag int

const (
	flagUpload  = 0x01
	flagComics  = 0x02
	flagBanning = 0x04
	flagDelete  = 0x08
	flagTags    = 0x16
	flagSpecial = 0x32

	flagAll = 0xff
)

func (f flag) Upload() bool {
	return f&flagUpload != 0
}

func (f flag) Comics() bool {
	return f&flagComics != 0
}

func (f flag) Banning() bool {
	return f&flagBanning != 0
}

func (f flag) Delete() bool {
	return f&flagDelete != 0
}

func (f flag) Tags() bool {
	return f&flagTags != 0
}

func (f flag) Special() bool {
	return f&flagSpecial != 0
}

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

func (p *Pool) QTitle(q querier) string {
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

func (p *Pool) QUser(q querier) *User {
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

func (p *Pool) QDescription(q querier) string {
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

func (p *Pool) QPosts(q querier) error {
	if len(p.Posts) > 0 {
		return nil
	}
	rows, err := q.Query("SELECT post_id, position FROM pool_mappings WHERE pool_id = $1 ORDER BY position, post_id DESC", p.ID)
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

func (p *Pool) Save(q querier) error {
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

func (p *Pool) Add(postID, position int) error {
	if p.ID <= 0 {
		return errors.New("Pool id is 0")
	}

	if postID <= 0 {
		return errors.New("Post id is < 0")
	}

	_, err := DB.Exec("INSERT INTO pool_mappings(pool_id, post_id, position) VALUES($1, $2, $3)", p.ID, postID, position)
	return err
}

func (u *User) QID(q querier) int {
	if u.ID != 0 {
		return u.ID
	}
	if u.Name != "" {
		err := q.QueryRow("SELECT id FROM users WHERE username=$1", u.Name).Scan(&u.ID)
		if err != nil {
			log.Print(err)
		}
	}
	if u.Session.userID != 0 {
		u.ID = u.Session.userID
	}

	return u.ID
}

func (u *User) SetID(id int) {
	u.ID = id
}

func (u *User) RecentPosts(q querier, limit int) []*Post {
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

func (u *User) QName(q querier) string {
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

func (u *User) SetFlag(f flag) {
	u.flag = &f
}

func (u *User) QFlag(q querier) flag {
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

func (u *User) Salt(q querier) string {
	return u.salt
}

func (u *User) PassHash(q querier) string {
	if u.passwordHash != "" {
		return u.passwordHash
	}

	if u.QID(q) == 0 {
		return ""
	}

	err := q.QueryRow("SELECT passwordhash, salt FROM users WHERE id=$1", u.QID(q)).Scan(&u.passwordHash, &u.salt)
	if err != nil {
		log.Print(err)
	}

	return u.passwordHash
}

func (u *User) Login(q querier, name, password string) error {
	u.Name = name

	if u.PassHash(q) == "" {
		return errors.New("err")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PassHash(q)), []byte(password+u.Salt(q))); err != nil {
		return err
	}
	err := u.Session.create(u)
	//err := sess.create(u)

	return err
}

func (u *User) Logout(q querier) error {
	u.Session.destroy(q)
	return nil
}

func (u User) Register(name, password string) error {
	u.Name = name
	var err error
	u.salt, err = createSalt()
	if err != nil {
		return err
	}

	u.SetFlag(flag(CFG.StdUserFlag))

	hash, err := bcrypt.GenerateFromPassword([]byte(password+u.salt), 0)
	if err != nil {
		return err
	}
	u.passwordHash = string(hash)

	tx, err := DB.Begin()
	if err != nil {
		log.Print(err)
		return err
	}
	var id int
	err = tx.QueryRow("SELECT id FROM users WHERE LOWER(username) = LOWER($1)", u.Name).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
		return txError(tx, err)
	}

	if id != 0 {
		// TODO: proper error
		return txError(tx, errors.New("Username already exist"))
	}

	_, err = tx.Exec("INSERT INTO users(username, passwordhash, salt, datejoined) VALUES($1, $2, $3, CURRENT_TIMESTAMP)", u.Name, u.passwordHash, u.salt)

	if err != nil {
		log.Print(err)
		return txError(tx, err)
	}

	err = tx.Commit()

	return err
}

func (u *User) QPools(q querier) []*Pool {
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

//func (u *User) QPoolsLimit(q querier, limit, offset int) []*Pool {
//	rows, err := q.Query("SELECT id, title, description FROM user_pools WHERE user_id = $1 LIMIT $2 OFFSET $3", u.QID(q), limit, offset*limit)
//	if err != nil {
//		log.Println(err)
//		return nil
//	}
//	defer rows.Close()
//
//	var pools []*Pool
//	for rows.Next() {
//		var pool Pool
//		pool.User = u
//		err = rows.Scan(&pool.ID, &pool.Title, &pool.Description)
//		if err != nil {
//			log.Println(err)
//			return nil
//		}
//		pools = append(pools, &pool)
//	}
//
//	if rows.Err() != nil {
//		log.Println(rows.Err())
//		return nil
//	}
//
//	return pools
//}

func (u *User) Sessions(q querier) []*Session {
	rows, err := q.Query("SELECT user_id, sesskey, to_char(expire, 'YYYY-MM-DD HH24:MI:SS') FROM sessions WHERE user_id=$1", u.QID(q))
	if err != nil {
		log.Println(err)
		return nil
	}

	var sessions []*Session

	for rows.Next() {
		s := &Session{}
		var t string
		if err = rows.Scan(&s.userID, &s.key, &t); err != nil {
			log.Println(err)
			return nil
		}

		s.expire, err = time.Parse(sqlite3Timestamp, t)
		if err != nil {
			log.Println(err)
		}

		sessions = append(sessions, s)
	}

	return sessions
}

type Session struct {
	userID int
	key    string
	expire time.Time
}

func (s *Session) Expire() time.Time {
	return s.expire
}

func (s *Session) UserID(q querier) int {
	if s.userID != 0 {
		return s.userID
	}

	if s.key == "" {
		return 0
	}

	err := q.QueryRow("SELECT user_id FROM sessions WHERE sesskey=$1", s.key).Scan(&s.userID)
	if err != nil {
		log.Print(err)
	}
	return s.userID
}

func (s *Session) Key(q querier) string {
	if s.key != "" {
		return s.key
	}

	if s.UserID(q) == 0 {
		return ""
	}

	return s.key
}

func (s *Session) create(user *User) error {
	s.key = sessionCreate(user.QID(DB))
	// s.key = randomString(64)
	// sessions[s.key] = user.ID()

	// _, err := q.Exec("INSERT INTO sessions(user_id, sesskey, expire) VALUES($1, $2, DATETIME(CURRENT_TIMESTAMP, '+30 days'))", user.ID(), s.key)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	return nil
}

func (s *Session) Get(q querier, key string) {
	// s.userID = sessions[key]
	s.userID = sessionRead(key)
	if s.userID == 0 {
		err := q.QueryRow("SELECT user_id FROM sessions WHERE sesskey=$1", key).Scan(&s.userID)
		if err != nil {
			return
		}
		sessionCreateNew(key, s.userID)
		// exp, err := time.Parse("2006-01-02 15:04:05", expire)
		// if err != nil {
		// 	log.Println(err)
		// 	s.key = ""
		// 	return
		// }
		// if exp.UnixNano() < time.Now().UnixNano() {
		// 	// Session has expired
		// 	q.Exec("DELETE FROM sessions WHERE sesskey=$1", key)
		// 	s.key = ""
		// 	return
		// }
		// sessions[key] = s.userID
	}
	s.key = key
}

func (s *Session) destroy(q querier) {
	sessionDestroy(s.Key(q))
	if s.Key(q) != "" {
		q.Exec("DELETE FROM sessions WHERE sesskey=$1", s.Key(q))
	}
}

func createSalt() (string, error) {
	b := make([]byte, 64)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", b), nil
}

var alp = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func randomString(n int) string {
	var str string
	for i := 0; i < n; i++ {
		str += string(alp[mrand.Intn(len(alp))])
	}
	return str
}

var (
	AdmFUser  = 0
	AdmFAdmin = 1
)

func UpgradeUserToAdmin(userID int) error {
	_, err := DB.Exec("UPDATE users SET adminflag=1 WHERE id=$1", userID)
	return err
}

func GetUsernameFromID(userID int) string {
	var username string
	err := DB.QueryRow("SELECT username FROM users WHERE id=$1", userID).Scan(&username)
	if err != nil {
		return "Anonymous"
	}
	return username
}

// TODO: In memory sessions have a lifespan of 30min, when it expires put/update the session in the database for 30 days
var sessions = make(map[string]int)

var sesmap = make(map[string]*list.Element)
var seslist = list.New()
var lock sync.Mutex

type sessions2 struct {
	sessions map[string]int
	sesmap   map[string]*list.Element
	seslist  *list.List
	l        sync.Mutex
}

func newSessions() *sessions2 {
	var s sessions2
	s.sessions = make(map[string]int)
	s.sesmap = make(map[string]*list.Element)
	seslist = list.New()
	return &s
}

type sess struct {
	lastAccess time.Time
	key        string
	userID     int
}

func sessionRead(key string) int {
	if e, ok := sesmap[key]; ok {
		sessionUpdate(key)
		return e.Value.(*sess).userID
	}
	return 0
}

func sessionCreateNew(key string, userID int) {
	lock.Lock()
	defer lock.Unlock()

	s := &sess{time.Now(), key, userID}

	e := seslist.PushBack(s)
	sesmap[key] = e
}

func sessionUpdate(key string) {
	lock.Lock()
	defer lock.Unlock()

	if e, ok := sesmap[key]; ok {
		e.Value.(*sess).lastAccess = time.Now()
		seslist.MoveToFront(e)
	}
}

func sessionCreate(userID int) string {
	lock.Lock()
	defer lock.Unlock()

	s := &sess{time.Now(), randomString(64), userID}
	e := seslist.PushBack(s)
	sesmap[s.key] = e

	_, err := DB.Exec("INSERT INTO sessions(user_id, sesskey, expire) VALUES($1, $2, CURRENT_TIMESTAMP + INTERVAL '30 DAY')", userID, s.key)
	if err != nil {
		log.Println(err)
	}
	return s.key
}

func sessionDestroy(key string) {
	if e, ok := sesmap[key]; ok {
		delete(sesmap, key)
		seslist.Remove(e)
	}
}

func sessionGC() {
	lock.Lock()
	defer lock.Unlock()
	for {
		e := seslist.Back()
		if e == nil {
			break
		}

		if (e.Value.(*sess).lastAccess.Add(time.Hour).Unix()) < time.Now().Unix() {
			seslist.Remove(e)
			delete(sesmap, e.Value.(*sess).key)
			_, err := DB.Exec("INSERT INTO sessions(user_id, sesskey, expire) VALUES($1, $2, CURRENT_TIMESTAMP + INTERVAL '30 DAY') ON CONFLICT (sesskey) DO UPDATE SET expire = CURRENT_TIMESTAMP + INTERVAL '30 DAY'", e.Value.(*sess).userID, e.Value.(*sess).key)
			if err != nil {
				log.Print(err)
			}
		} else {
			break
		}
	}
	_, err := DB.Exec("DELETE FROM sessions WHERE expire < CURRENT_TIMESTAMP")
	if err != nil {
		log.Println(err)
	}

	time.AfterFunc(time.Minute*15, sessionGC)
}
