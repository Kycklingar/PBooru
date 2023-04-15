package user

import (
	"context"
	"time"

	Cache "github.com/kycklingar/PBooru/DataManager/cache"
)

var cache = Cache.NewGeneric[ID, User](User{}, time.Minute*30, time.Hour*24)
var profileCache = Cache.NewGeneric[ID, Profile](Profile{}, time.Minute*30, time.Hour*24)

func init() {
	go cache.GC(context.Background())
	go profileCache.GC(context.Background())
}
