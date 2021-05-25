package DataManager

import "database/sql"

type DnsCreator struct {
	Id    int
	Name  string
	Score int
	Tags  []DnsTag
	Urls  []DnsUrl
}

type DnsTag struct {
	Id          string
	Name        string
	Description string
	Score       int
}

type DnsUrl struct {
	Url    string
	Domain string
}

func ListDnsCreators() ([]DnsCreator, error) {
	rows, err := DB.Query(`
		SELECT c.id, c.name, c.score,
			dt.id, dt.name, dt.description, dt.score
		FROM (
			SELECT id, name, score
			FROM dns_creator_scores
			ORDER BY score DESC
			LIMIT 10
		) c
		LEFT JOIN dns_tags dts
		ON c.id = dts.creator_id
		LEFT JOIN dns_tag dt
		ON dts.tag_id = dt.id
		ORDER BY c.score DESC
		`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creators []DnsCreator

	for rows.Next() {
		var (
			creator = new(DnsCreator)
			tid     sql.NullString
			tname   sql.NullString
			tdesc   sql.NullString
			tscore  sql.NullInt32
		)

		err = rows.Scan(
			&creator.Id,
			&creator.Name,
			&creator.Score,
			&tid,
			&tname,
			&tdesc,
			&tscore,
		)
		if err != nil {
			return nil, err
		}

		if len(creators) <= 0 || creators[len(creators)-1].Id != creator.Id {
			creators = append(creators, *creator)
		}
		creator = &creators[len(creators)-1]

		if tid.Valid {
			var tag = DnsTag{
				tid.String,
				tname.String,
				tdesc.String,
				int(tscore.Int32),
			}

			creator.Tags = append(creator.Tags, tag)
		}
	}

	return creators, nil
}

func GetDnsCreator(id int) (c DnsCreator, err error) {
	err = DB.QueryRow(`
		SELECT id, name, score
		FROM dns_creator_scores
		WHERE id = $1
		`,
		id,
	).Scan(&c.Id, &c.Name, &c.Score)
	if err != nil {
		return
	}

	rows, err := DB.Query(`
		SELECT url, d.domain
		FROM dns_creator_urls u
		JOIN dns_domain d
		ON u.domain = d.id
		WHERE u.id = $1
		`,
		id,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var u DnsUrl
		err = rows.Scan(&u.Url, &u.Domain)
		if err != nil {
			return
		}
		c.Urls = append(c.Urls, u)
	}

	return
}
