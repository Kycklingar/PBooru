package migrate

import "github.com/kycklingar/PBooru/DataManager/db"

type ExecQuery interface {
	db.Q
}
