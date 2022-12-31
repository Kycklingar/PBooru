package DataManager

import (
	"context"
	"log"

	mm "github.com/kycklingar/MinMax"
	"github.com/kycklingar/PBooru/DataManager/user"
	"github.com/kycklingar/PBooru/DataManager/user/pool"
)

type Pool struct {
	pool.Pool

	Posts []PoolMapping
}

type PoolMapping struct {
	Post     *Post
	Position int
}

// Until posts refactor
func PoolFromID(ctx context.Context, id pool.ID) (Pool, error) {
	p, err := pool.FromID(ctx, id)
	return Pool{Pool: p}, err
}

func PoolsOfUser(ctx context.Context, id user.ID) ([]Pool, error) {
	ppools, err := pool.OfUser(ctx, id)

	var pools []Pool
	for _, pool := range ppools {
		pools = append(pools, Pool{Pool: pool})
	}

	return pools, err
}

func (p *Pool) QPosts(q querier) error {
	if len(p.Posts) > 0 {
		return nil
	}
	rows, err := q.Query("SELECT post_id, position FROM pool_mappings WHERE pool_id = $1 ORDER BY (position, post_id) DESC", p.ID)
	if err != nil {
		log.Println(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var pm PoolMapping
		pm.Post = NewPost()
		err = rows.Scan(&pm.Post.ID, &pm.Position)
		if err != nil {
			log.Println(err)
			return err
		}

		p.Posts = append(p.Posts, pm)
	}

	return rows.Err()
}

func (p *Pool) PostsLimit(limit int) []PoolMapping {
	return p.Posts[:mm.Min(limit, len(p.Posts))]
}
