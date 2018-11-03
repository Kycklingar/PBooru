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
	u.AdmFlag = -1
	u.Session = &Session{}
	return &u
}

type User struct {
	ID           int
	Name         string
	passwordHash string
	salt         string
	Joined       time.Time
	AdmFlag      int
	Session      *Session
}

func (u *User) QID(q querier) int {
	if u.ID != 0 {
		return u.ID
	}
	if u.Name != "" {
		err := q.QueryRow("SELECT id FROM users WHERE username=?", u.Name).Scan(&u.ID)
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

func (u *User) QName(q querier) string {
	if u.Name != "" {
		return u.Name
	}
	if u.QID(q) == 0 {
		return ""
	}
	err := q.QueryRow("SELECT username FROM users WHERE id=?", u.QID(q)).Scan(&u.Name)
	if err != nil {
		log.Print(err)
	}
	return u.Name
}

func (u *User) SetName(name string) {
	u.Name = name
}

func (u *User) QFlag(q querier) int {
	if u.AdmFlag != -1 {
		return u.AdmFlag
	}
	if u.QID(q) == 0 {
		return -1
	}

	err := q.QueryRow("SELECT adminflag FROM users WHERE id=?", u.QID(q)).Scan(&u.AdmFlag)
	if err != nil {
		log.Print(err)
	}
	return u.AdmFlag
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

	err := q.QueryRow("SELECT passwordhash, salt FROM users WHERE id=?", u.QID(q)).Scan(&u.passwordHash, &u.salt)
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
	err = tx.QueryRow("SELECT id FROM users WHERE username=LOWER(?)", u.Name).Scan(&id)
	if err != nil && err != sql.ErrNoRows {
		log.Print(err)
		return txError(tx, err)
	}

	if id != 0 {
		// TODO: proper error
		return txError(tx, errors.New("Username already exist"))
	}

	_, err = tx.Exec("INSERT INTO users(username, passwordhash, salt, datejoined) VALUES(?, ?, ?, CURRENT_TIMESTAMP)", u.Name, u.passwordHash, u.salt)

	if err != nil {
		log.Print(err)
		return txError(tx, err)
	}

	err = tx.Commit()

	return err
}

func (u *User) Sessions(q querier) []*Session {
	rows, err := q.Query("SELECT * FROM sessions WHERE user_id=?", u.QID(q))
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

		s.expire, _ = time.Parse(Sqlite3Timestamp, t)

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

	err := q.QueryRow("SELECT user_id FROM sessions WHERE sesskey=?", s.key).Scan(&s.userID)
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

	// _, err := q.Exec("INSERT INTO sessions(user_id, sesskey, expire) VALUES(?, ?, DATETIME(CURRENT_TIMESTAMP, '+30 days'))", user.ID(), s.key)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	return nil
}

func (s *Session) Get(q querier, key string) {
	// s.userID = sessions[key]
	s.userID = sessionRead(key)
	if s.userID == 0 {
		err := q.QueryRow("SELECT user_id FROM sessions WHERE sesskey=?", key).Scan(&s.userID)
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
		// 	q.Exec("DELETE FROM sessions WHERE sesskey=?", key)
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
		q.Exec("DELETE FROM sessions WHERE sesskey=?", s.Key(q))
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
	_, err := DB.Exec("UPDATE users SET adminflag=1 WHERE id=?", userID)
	return err
}

func GetUsernameFromID(userID int) string {
	var username string
	err := DB.QueryRow("SELECT username FROM users WHERE id=?", userID).Scan(&username)
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

	_, err := DB.Exec("INSERT INTO sessions(user_id, sesskey, expire) VALUES(?, ?, DATE_ADD(CURRENT_TIMESTAMP, INTERVAL 30 DAY))", userID, s.key)
	if err != nil {
		fmt.Println(err)
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
			_, err := DB.Exec("INSERT INTO sessions(user_id, sesskey, expire) VALUES(?, ?, DATE_ADD(CURRENT_TIMESTAMP, INTERVAL 30 DAY)) ON DUPLICATE KEY UPDATE expire = DATE_ADD(CURRENT_TIMESTAMP, INTERVAL 30 DAY)", e.Value.(*sess).userID, e.Value.(*sess).key)
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
