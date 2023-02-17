package user

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/session"
	"github.com/kycklingar/PBooru/DataManager/timestamp"
	"github.com/kycklingar/PBooru/DataManager/user/flag"
	"golang.org/x/crypto/bcrypt"
)

type ID = int

var ErrWrongCredentials = errors.New("wrong credentials")

type User struct {
	ID     ID
	Name   string
	Joined timestamp.Timestamp
	Flag   flag.Flag

	Title string
}

func (u *User) Scan(value any) (err error) {
	v, ok := value.(int64)
	if !ok {
		return errors.New("scan incorrect type")
	}

	// probably not a good idea to do here, but its convenient
	*u, err = fetchUser(context.Background(), ID(v))
	return
}

func Login(ctx context.Context, username, password string) (key session.Key, user User, err error) {
	var (
		hash string
		salt string
	)

	err = db.Context.QueryRowContext(
		ctx,
		`SELECT user_id, hash, salt
		FROM passwords
		JOIN users
		ON user_id = users.id
		WHERE username = $1`,
		username,
	).Scan(&user.ID, &hash, &salt)
	if err != nil {
		if err == sql.ErrNoRows {
			err = ErrWrongCredentials
		}
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password+salt))
	if err != nil {
		err = ErrWrongCredentials
		return
	}

	key, err = keeper.Keep(user.ID)
	if err != nil {
		return
	}

	user, err = fetchUser(ctx, user.ID)
	return
}

func Logout(key session.Key) error {
	return keeper.Destroy(key)
}

func Register(ctx context.Context, username, password string, privileges flag.Flag) error {
	salt, err := createSalt()
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password+salt), 0)
	if err != nil {
		return err
	}

	tx, err := db.Context.Begin()
	if err != nil {
		return err
	}

	var id ID
	err = tx.QueryRowContext(
		ctx,
		`INSERT INTO users(username, adminflag)
		VALUES($1, $2)
		RETURNING id`,
		username,
		privileges,
	).Scan(&id)
	if err != nil {
		tx.Rollback()
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO passwords(user_id, hash, salt)
		VALUES($1, $2, $3)`,
		id,
		string(hash),
		salt,
	)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func FromSession(ctx context.Context, key session.Key) (user User, err error) {
	var id any
	id, err = keeper.Get(key)
	if err != nil {
		return
	}

	return fetchUser(ctx, id.(ID))
}

func FromID(ctx context.Context, id ID) (User, error) {
	return fetchUser(ctx, id)
}

// fetches user from cache or db
func fetchUser(ctx context.Context, id ID) (User, error) {
	user, ok := cache.Get(id)
	if ok {
		return user, nil
	}

	var logCount int

	err := db.Context.QueryRowContext(
		ctx,
		`SELECT username, adminflag, datejoined, count(log_id)
		FROM users
		LEFT JOIN logs
		ON id = user_id
		WHERE id = $1
		GROUP BY id`,
		id,
	).Scan(&user.Name, &user.Flag, &user.Joined, &logCount)

	user.ID = id
	user.Title = title(logCount)

	cache.Set(id, user)

	return user, err
}
