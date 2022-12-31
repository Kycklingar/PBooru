package inbox

import "strings"

const (
	sqlSelectMessage = `SELECT id, sender, recipient, title, text, date
			FROM message`

	sqlSelectCount = `SELECT count(*)
			FROM message`

	sqlWhereID = `WHERE id = $1`

	sqlWhereUnread = `LEFT JOIN messages_read
			ON id = message_id
			WHERE recipient = $1
			AND message_id IS NULL`

	sqlWhereSent = `JOIN messages_sent
			ON id = message_id
			WHERE sender = $1`

	sqlWhereAll = `LEFT JOIN messages
			ON id = message_id
			WHERE message_id IS NOT NULL
			AND recipient = $1`

	sqlOrderByDate = `ORDER BY date DESC`
)

var (
	sqlMessageByID = strings.Join(
		[]string{
			sqlSelectMessage,
			sqlWhereID,
		},
		"\n",
	)
	sqlAllMessages = strings.Join(
		[]string{
			sqlSelectMessage,
			sqlWhereAll,
			sqlOrderByDate,
		},
		"\n",
	)
	sqlAllCount = strings.Join(
		[]string{
			sqlSelectCount,
			sqlWhereAll,
		},
		"\n",
	)
	sqlUnreadMessages = strings.Join(
		[]string{
			sqlSelectMessage,
			sqlWhereUnread,
			sqlOrderByDate,
		},
		"\n",
	)
	sqlUnreadCount = strings.Join(
		[]string{
			sqlSelectCount,
			sqlWhereUnread,
		},
		"\n",
	)
	sqlSentMessages = strings.Join(
		[]string{
			sqlSelectMessage,
			sqlWhereSent,
			sqlOrderByDate,
		},
		"\n",
	)
	sqlSentCount = strings.Join(
		[]string{
			sqlSelectCount,
			sqlWhereSent,
		},
		"\n",
	)
)
