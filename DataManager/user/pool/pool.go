package pool

import (
	"context"

	"github.com/kycklingar/PBooru/DataManager/db"
	"github.com/kycklingar/PBooru/DataManager/user"
)

type ID int

type Pool struct {
	ID          ID
	User        user.User
	Title       string
	Description string
}

func (p Pool) AddPost(ctx context.Context, postID int) error {
	_, err := db.Context.ExecContext(
		ctx,
		`INSERT INTO pool_mappings(pool_id, post_id, position)
		VALUES(
			$1, $2,
			COALESCE(
				(SELECT MAX(position) + 1
				FROM pool_mappings
				WHERE pool_id = $1),
				0
			)
		)`,
		p.ID,
		postID,
	)

	return err
}

func (p Pool) RemovePost(ctx context.Context, postID int) error {
	_, err := db.Context.ExecContext(
		ctx,
		`DELETE FROM pool_mappings
		WHERE pool_id = $1
		AND post_id = $2`,
		p.ID,
		postID,
	)
	return err
}

func (p *Pool) scan(scan db.Scanner) error {
	return scan(
		&p.ID,
		&p.User,
		&p.Title,
		&p.Description,
	)
}

func Create(ctx context.Context, userID user.ID, title, description string) error {
	_, err := db.Context.ExecContext(
		ctx,
		`INSERT INTO user_pools(user_id, title, description)
		VALUES($1, $2, $3)`,
		userID,
		title,
		description,
	)

	return err
}

func OfUser(ctx context.Context, id user.ID) (Pools, error) {
	var pools Pools
	return pools, db.QueryRowsContext(
		ctx, db.Context,
		`SELECT id, user_id, title, description
		FROM user_pools
		WHERE user_id = $1`,
		id,
	)(pools.scan)
}

func FromID(ctx context.Context, id ID) (Pool, error) {
	var pool Pool
	return pool, pool.scan(db.Context.QueryRowContext(
		ctx,
		`SELECT id, user_id, title, description
		FROM user_pools
		WHERE id = $1`,
		id,
	).Scan)
}
