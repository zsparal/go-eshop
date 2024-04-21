package events

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/zsparal/reporting/internal/events/db"
)

type PayloadEvent interface {
	SerializePayload() []byte
}

type EventToAdd struct {
	StreamID        StreamID
	IdempotencyKey  IdempotencyKey
	ExpectedVersion Version
	Payload         PayloadEvent
}

type Event struct {
	streamID   StreamID
	version    Version
	recordedAt time.Time
	payload    []byte
}

func (e *Event) StreamID() StreamID {
	return e.streamID
}

func (e *Event) Version() Version {
	return e.version
}

func (e *Event) Payload() []byte {
	return e.payload
}

func newEventFromDatabase(e db.Event) (Event, error) {
	if e.Version < 0 {
		return Event{}, ErrInvalidStreamVersion
	}

	if !e.RecordedAt.Valid || e.RecordedAt.InfinityModifier != pgtype.Finite {
		return Event{}, ErrInvalidEventTimestamp
	}

	streamID := StreamID(e.StreamID)
	version := Version(e.Version)
	payload := e.Payload
	recordedAt := e.RecordedAt.Time

	return Event{streamID, version, recordedAt, payload}, nil
}
