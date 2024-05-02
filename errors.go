package glow

import "errors"

var (
	ErrEmptyNetwork       = errors.New("network is empty")
	ErrNetworkNeedPurging = errors.New("network needs purging")

	ErrNodeNotFound      = errors.New("node not found")
	ErrBadNodeKey        = errors.New("bad node key")
	ErrNodeAlreadyExists = errors.New("node already exists")
	ErrNodeIsConnected   = errors.New("node is connected")
	ErrSeedingDone       = errors.New("seeding is done")
	ErrNodeGoingAway     = errors.New("node is going away")
	ErrIsolatedNodeFound = errors.New("isolated node found")

	ErrLinkNotFound      = errors.New("link not found")
	ErrLinkAlreadyExists = errors.New("link already exists")
	ErrCyclesNotAllowed  = errors.New("cycles not allowed")
	ErrLinkAlreadyPaused = errors.New("link already paused")
)
