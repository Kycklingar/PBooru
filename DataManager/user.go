package DataManager

import (
	"container/list"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	mrand "math/rand"
	"strconv"
	"sync"
	"time"

	C "github.com/kycklingar/PBooru/DataManager/cache"
	"golang.org/x/crypto/bcrypt"
)

func NewUser() *User {
	var u User
	u.Session = &Session{}
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

type User struct {
	ID           int
	Name         string
	passwordHash string
	salt         string
	Joined       time.Time
	flag         *flag
	Session      *Session

	Messages messages

	editCount *int

	Pools []*Pool
}

type flag int

const (
	flagTagging = 1
	flagUpload  = 2
	flagComics  = 4
	flagBanning = 8
	flagDelete  = 16
	flagTags    = 32
	flagSpecial = 64

	flagAll = 0xff
)

func (f flag) Tagging() bool {
	return f&flagTagging != 0
}

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

func (u *User) tagEditCount(q querier) int {
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

func (u *User) SetID(q querier, id int) error {
	if u.ID > 0 {
		return nil
	}
	if err := q.QueryRow("SELECT id FROM users WHERE id = $1", id).Scan(&u.ID); err != nil {
		log.Println(err)
		return err
	}

	return nil
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

func (u *User) SetFlag(q querier, f int) error {
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

func (u *User) Voted(q querier, p *Post) bool {
	if p.QID(q) <= 0 {
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

	u.flag = new(flag) //(CFG.StdUserFlag))
	*u.flag = flag(CFG.StdUserFlag)

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

	_, err = tx.Exec("INSERT INTO users(username, passwordhash, salt, datejoined, adminflag) VALUES($1, $2, $3, CURRENT_TIMESTAMP, $4)", u.Name, u.passwordHash, u.salt, u.flag)

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

func (u *User) RecentVotes(q querier, limit int) (posts []*Post) {
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
