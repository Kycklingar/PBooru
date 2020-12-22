package DataManager

import (
	mrand "math/rand"
	"time"

	"github.com/kycklingar/PBooru/DataManager/querier"
)

type Session struct {
	User     int
	Key      string
	Accessed time.Time
}

const (
	insertQuery = `
		INSERT INTO sessions(user_id, sesskey, expire)
		VALUES($1, $2, CURRENT_TIMESTAMP + INTERVAL '30 DAY')
		`
	sessionCacheLifetime = time.Hour
	randRunes            = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

func sessionNew(q querier.Q, user int) (string, error) {
	s := Session{
		User:     user,
		Key:      randomString(64),
		Accessed: time.Now(),
	}

	sc.Push(s)

	_, err := q.Exec(insertQuery, s.User, s.Key)

	return s.Key, err
}

func sessionGet(q querier.Q, key string) (int, error) {
	if s := sc.Get(key); s != nil {
		sc.Update(key)
		return s.User, nil
	}

	var s = Session{
		Accessed: time.Now(),
	}

	err := q.QueryRow(`
		SELECT user_id, sesskey
		FROM sessions
		WHERE sesskey = $1
		`,
		key,
	).Scan(&s.User, &s.Key)
	if err != nil {
		return 0, err
	}

	sc.Push(s)

	return s.User, nil
}

func sessionDestroy(q querier.Q, key string) error {
	sc.Purge(key)

	_, err := q.Exec(`
		DELETE FROM sessions
		WHERE sesskey = $1
		`,
		key,
	)

	return err
}

func sessionGC(q querier.Q) error {
	for s := sc.Last(); s != nil && s.Accessed.Before(time.Now().Add(sessionCacheLifetime)); s = sc.Last() {
		if _, err := q.Exec(insertQuery+`
			ON CONFLICT (sesskey)
			DO UPDATE SET expire = CURRENT_TIMESTAMP + INTERVAL '30 DAY'
			`,
			s.User,
			s.Key,
		); err != nil {
			return err
		}

		sc.Purge(s.Key)
	}

	_, err := q.Exec(`
		DELETE FROM sessions
		WHERE expire < CURRENT_TIMESTAMP
		`,
	)
	return err
}

func randomString(n int) string {
	var str string
	for i := 0; i < n; i++ {
		str += string(randRunes[mrand.Intn(len(randRunes))])
	}
	return str
}
