package user

import (
	"context"
	"time"

	Cache "github.com/kycklingar/PBooru/DataManager/cache"
)

var cache = Cache.NewGeneric[ID, User](User{}, time.Minute*30)

func init() {
	go cache.GC(context.Background())
}
