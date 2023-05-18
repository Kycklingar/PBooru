package user

import (
	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/user/flag"
)

func SetPrivileges(q db.Q, id ID, privilege flag.Flag) error {
	_, err := q.Exec(
		`UPDATE users
		SET adminflag = $1
		WHERE id = $2`,
		privilege,
		id,
	)

	cache.Del(id)
	profileCache.Del(id)

	return err
}
