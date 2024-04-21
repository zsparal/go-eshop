package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/zsparal/reporting/internal/core"
	"github.com/zsparal/reporting/internal/events/db"
)

type EventsRepository interface {
	GetStream(ctx context.Context, streamID StreamID) (Stream, error)
	CreateStream(ctx context.Context, streamToCreate StreamToCreate) (Stream, error)
	GetEvents(ctx context.Context, streamID StreamID) ([]Event, error)
	AddEventToStream(ctx context.Context, eventToAdd EventToAdd) (Event, error)
}

type dbEventsRepository struct {
	db core.DatabaseConnection
}

func NewRepository(db core.DatabaseConnection) EventsRepository {
	return &dbEventsRepository{db}
}

func (r *dbEventsRepository) GetStream(ctx context.Context, streamID StreamID) (Stream, error) {
	queries := db.New(r.db)
	s, err := queries.GetStream(ctx, uuid.UUID(streamID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Stream{}, fmt.Errorf("%w with stream id: %s", ErrStreamNotFound, streamID)
	}

	if err != nil {
		return Stream{}, fmt.Errorf("%w for stream %s because: %w", ErrFailedToGetStream, streamID, err)
	}

	return newStreamFromDatabase(s)
}

func (r *dbEventsRepository) CreateStream(ctx context.Context, streamToCreate StreamToCreate) (Stream, error) {
	queries := db.New(r.db)
	s, err := queries.CreateStream(ctx, db.CreateStreamParams{
		StreamID:       uuid.UUID(streamToCreate.StreamID),
		IdempotencyKey: uuid.UUID(streamToCreate.IdempotencyKey),
	})
	if err != nil {
		return Stream{}, ErrStreamAlreadyExists
	}

	return newStreamFromDatabase(s)
}

func (r *dbEventsRepository) GetEvents(ctx context.Context, streamID StreamID) ([]Event, error) {
	queries := db.New(r.db)
	dbEvents, err := queries.GetEventsForStream(ctx, uuid.UUID(streamID))
	if err != nil {
		return []Event{}, fmt.Errorf("%w for stream %s because: %w", ErrCouldNotFetchEventStream, streamID, err)
	}

	events := make([]Event, len(dbEvents))
	for _, dbEvent := range dbEvents {
		event, err := newEventFromDatabase(dbEvent)
		if err != nil {
			return []Event{}, err
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *dbEventsRepository) AddEventToStream(ctx context.Context, eventToAdd EventToAdd) (Event, error) {
	result, err := core.InTransaction(ctx, r.db, func(tx core.DatabaseConnection) (Event, error) {
		queries := db.New(tx)

		// First, we try and increment the stream version. Since this is an update, this will lock the respective row as well,
		// making sure that if we succeed, then we are the one who can append to the event stream
		stream, err := queries.IncrementStreamVersion(ctx, db.IncrementStreamVersionParams{
			StreamID:        uuid.UUID(eventToAdd.StreamID),
			ExpectedVersion: int64(eventToAdd.ExpectedVersion),
		})

		if err != nil {
			return Event{}, fmt.Errorf("%w for stream %s using expected version %d because: %w", ErrCouldNotIncrementStreamVersion, eventToAdd.StreamID, eventToAdd.ExpectedVersion, err)
		}

		// Now, we can insert to dbEvent to the dbEvent stream
		dbEvent, err := queries.AddEventToStream(ctx, db.AddEventToStreamParams{
			StreamID:       uuid.UUID(eventToAdd.StreamID),
			IdempotencyKey: uuid.UUID(eventToAdd.IdempotencyKey),
			Version:        stream.LatestVersion,
			Payload:        eventToAdd.Payload.SerializePayload(),
		})

		if err != nil {
			return Event{}, fmt.Errorf("%w for stream %s because: %w", ErrFailedToInsertEventToStream, eventToAdd.StreamID, err)
		}

		event, err := newEventFromDatabase(dbEvent)

		if err != nil {
			return Event{}, fmt.Errorf("could not create domain event from database because: %w", err)
		}

		return event, nil
	})

	if errors.Is(err, core.ErrCouldNotStartTransaction) || errors.Is(err, core.ErrCouldNotCommitTransaction) {
		return result, fmt.Errorf("%w because of %w", ErrFailedToInsertEventToStream, err)
	}

	return result, err
}
