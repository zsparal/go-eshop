package events

import (
	"github.com/gofrs/uuid"
	"github.com/zsparal/reporting/internal/events/db"
)

type Version uint64
type StreamID uuid.UUID
type IdempotencyKey uuid.UUID

type StreamToCreate struct {
	StreamID       StreamID
	IdempotencyKey IdempotencyKey
}

type Stream struct {
	streamID       StreamID
	latestVersion  Version
	idempotencyKey IdempotencyKey
}

func (s *Stream) StreamID() StreamID {
	return s.streamID
}

func (s *Stream) LatestVersion() Version {
	return s.latestVersion
}

func (s *Stream) IdempotencyKey() IdempotencyKey {
	return s.idempotencyKey
}

func newStreamFromDatabase(s db.Stream) (Stream, error) {
	if s.LatestVersion < 0 {
		return Stream{}, ErrInvalidStreamVersion
	}

	streamID := StreamID(s.StreamID)
	latestVersion := Version(uint64(s.LatestVersion))
	idempotencyKey := IdempotencyKey(s.IdempotencyKey)

	return Stream{streamID, latestVersion, idempotencyKey}, nil
}
