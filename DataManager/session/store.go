package session

import "errors"

var NotFound = errors.New("key not found in store")

// Store represent the long time storage backing for sessions
type Store interface {
	Retrieve(Key) (any, error)

	// Store a new session
	Put(Session) error

	// Remove session
	Erase(Key) error

	// Refresh the expire of these sessions
	UpdateExpiration(...Session) error

	// Remove stale sessions
	CollectExpired() error
}
