package DataManager

import (
	"database/sql"
	"io"
	"sync"
	"time"

	"github.com/kycklingar/PBooru/DataManager/dns"
)

type DnsCreator struct {
	Id        int
	Name      string
	Score     int
	Tags      []DnsTag
	LocalTags []*Tag
	Domains   map[string]DnsDomain
	Banners   map[string]string
}

type DnsTag struct {
	Id          string
	Name        string
	Description string
	Score       int
}

type DnsDomain struct {
	Domain dns.Domain
	Urls   []dns.URL
}

func DnsNewBanner(file io.ReadSeeker, creatorID int, bannerType string) error {
	return dns.NewBanner(
		file,
		ipfs,
		store,
		DB,
		creatorID,
		bannerType,
	)
}

func ListDnsCreators(limit, offset int) ([]DnsCreator, error) {
	rows, err := DB.Query(`
		SELECT c.id, c.name, c.score,
			dt.id, dt.name, dt.description, dt.score
		FROM (
			SELECT id, name, score
			FROM dns_creator_scores
			ORDER BY score DESC
			LIMIT $1
			OFFSET $2
		) c
		LEFT JOIN dns_tags dts
		ON c.id = dts.creator_id
		LEFT JOIN dns_tag dt
		ON dts.tag_id = dt.id
		ORDER BY c.score DESC
		`,
		limit,
		offset,
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
			if err = creator.getBanners(); err != nil {
				return nil, err
			}
			if err = creator.getDomains(); err != nil {
				return nil, err
			}
			if err = creator.getLocalTags(); err != nil {
				return nil, err
			}

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

func (c *DnsCreator) getLocalTags() error {
	rows, err := DB.Query(`
		SELECT t.id, t.tag, n.nspace
		FROM dns_tag_mapping
		JOIN tags t
		on tag_id = t.id
		JOIN namespaces n
		ON namespace_id = n.id
		WHERE creator_id = $1
		`,
		c.Id,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var t = NewTag()

		err = rows.Scan(&t.ID, &t.Tag, &t.Namespace.Namespace)
		if err != nil {
			return err
		}

		c.LocalTags = append(c.LocalTags, t)
	}

	return nil
}

func (c *DnsCreator) getBanners() error {
	c.Banners = make(map[string]string)

	rows, err := DB.Query(`
		SELECT cid, banner_type
		FROM dns_banners
		WHERE creator_id = $1
		`,
		c.Id,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid string
			t   string
		)
		if err = rows.Scan(&cid, &t); err != nil {
			return err
		}

		c.Banners[t] = cid
	}

	return nil
}

func (c *DnsCreator) getDomains() error {
	c.Domains = make(map[string]DnsDomain)

	rows, err := DB.Query(`
			SELECT url, icon, d.domain
			FROM dns_creator_urls u
			JOIN dns_domain d
			ON u.domain = d.id
			WHERE u.id = $1
			`,
		c.Id,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			dom dns.Domain
			url dns.URL
		)

		err = rows.Scan(&url, &dom.Icon, &dom.Domain)
		if err != nil {
			return err
		}

		domain := c.Domains[dom.Domain]
		domain.Domain = dom

		domain.Urls = append(domain.Urls, url)
		c.Domains[dom.Domain] = domain
	}

	return nil
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

	if err = c.getDomains(); err != nil {
		return
	}

	err = func() error {
		rows, err := DB.Query(`
			SELECT id, name, description, score
			FROM dns_tag
			JOIN dns_tags 
			ON id = tag_id
			WHERE creator_id = $1
			`,
			c.Id,
		)
		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			var tag DnsTag

			err = rows.Scan(
				&tag.Id,
				&tag.Name,
				&tag.Description,
				&tag.Score,
			)
			if err != nil {
				return err
			}

			c.Tags = append(c.Tags, tag)
		}

		return nil
	}()
	if err != nil {
		return
	}

	if err = c.getBanners(); err != nil {
		return
	}

	err = c.getLocalTags()

	return
}

var dnsSpotlight struct {
	creator  *DnsCreator
	previous int
	l        sync.Mutex
}

func DnsSpotlight() (*DnsCreator, error) {
	dnsSpotlight.l.Lock()
	defer dnsSpotlight.l.Unlock()

	if dnsSpotlight.creator != nil {
		return dnsSpotlight.creator, nil
	}

	var id int

	err := DB.QueryRow(`
		SELECT id
		FROM dns_creator_scores
		WHERE score > -50
		AND id != $1
		ORDER BY RANDOM()
		`,
		dnsSpotlight.previous,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	dnsSpotlight.previous = id

	c, err := GetDnsCreator(id)
	if err != nil {
		return nil, err
	}

	dnsSpotlight.creator = &c

	time.AfterFunc(time.Hour*36, func() {
		dnsSpotlight.l.Lock()
		defer dnsSpotlight.l.Unlock()
		dnsSpotlight.creator = nil
	})

	return &c, nil
}

func DnsGetCreatorFromTag(tagId int) (DnsCreator, error) {
	var cid int
	err := DB.QueryRow(`
		SELECT creator_id
		FROM dns_tag_mapping
		WHERE tag_id = $1
		`,
		tagId,
	).Scan(&cid)
	if err != nil {
		return DnsCreator{}, err
	}

	return GetDnsCreator(cid)
}

func DnsMapTag(creatorID int, tagstr string) error {
	tag := NewTag()
	err := tag.Parse(tagstr)
	if err != nil {
		return err
	}

	tag.QID(DB)

	return dns.MapTag(DB, creatorID, tag.ID)
}
