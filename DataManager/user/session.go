package user

import (
	"context"
	"database/sql"
	"time"

	"github.com/kycklingar/PBooru/DataManager/session"
)

var sessionStore sessionStorage
var keeper *session.Keeper

func InitSession(db *sql.DB) {
	sessionStore = sessionStorage{db}
	keeper = session.NewKeeper(sessionStore, time.Hour/3)

	go keeper.GC(context.Background())
}

func ActiveSessions(id ID) ([]UserSession, error) {
	return sessionStore.userSessions(id)
}
