package user

import (
	"context"

	"github.com/kycklingar/PBooru/DataManager/db"
)

type Profile struct {
	User
	Title string
}

func (user User) Profile(ctx context.Context) (Profile, error) {
	profile, ok := profileCache.Get(user.ID)
	if !ok {
		var logCount int
		err := db.Context.QueryRowContext(
			ctx,
			`SELECT count(*)
			FROM logs
			WHERE user_id = $1`,
			user.ID,
		).Scan(&logCount)
		if err != nil {
			return profile, err
		}

		profile = Profile{User: user, Title: title(logCount)}

		profileCache.Set(user.ID, profile)
	}

	return profile, nil
}
