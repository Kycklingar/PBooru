package pool

import "github.com/kycklingar/PBooru/DataManager/db"

type Pools []Pool

func (pools *Pools) scan(scan db.Scanner) error {
	var pool Pool
	err := pool.scan(scan)
	*pools = append(*pools, pool)
	return err
}
