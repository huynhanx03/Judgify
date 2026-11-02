package realtime

import "errors"

var (
	ErrInvalidConfig        = errors.New("realtime: invalid configuration")
	ErrInvalidLifecycle     = errors.New("realtime: invalid lifecycle state")
	ErrTicketUnavailable    = errors.New("realtime: ticket unavailable")
	ErrTicketCapacity       = errors.New("realtime: ticket capacity reached")
	ErrTicketStoreClosed    = errors.New("realtime: ticket store closed")
	ErrInvalidPrincipal     = errors.New("realtime: invalid principal")
	ErrTopicForbidden       = errors.New("realtime: topic forbidden")
	ErrInvalidChange        = errors.New("realtime: invalid change")
	ErrInvalidReplayRequest = errors.New("realtime: invalid replay request")
	ErrReplayConflict       = errors.New("realtime: replay evidence conflict")
	ErrReplayUnavailable    = errors.New("realtime: replay unavailable")
	ErrPublisherUnavailable = errors.New("realtime: publisher unavailable")
	ErrHealthUnavailable    = errors.New("realtime: health unavailable")
)
