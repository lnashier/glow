package glow

import "errors"

var (
	ErrNodeNotFound      = errors.New("node not found")
	ErrBadNodeKey        = errors.New("bad node key")
	ErrNodeAlreadyExists = errors.New("node already exists")
	ErrLinkNotFound      = errors.New("link not found")
	ErrLinkAlreadyExists = errors.New("link already exists")
	ErrNodeIsConnected   = errors.New("node is connected")
	ErrEmptyNetwork      = errors.New("network is empty")
	ErrSeedingDone       = errors.New("seeding is done")
	ErrNodeGoingAway     = errors.New("node is going away")
	ErrIsolatedNodeFound = errors.New("isolated node found")
)
