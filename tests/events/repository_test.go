package events

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/zsparal/reporting/internal/events"
	ts "github.com/zsparal/reporting/tests/testing"
)

func TestMain(m *testing.M) {
	ts.TestEnvironment.Setup()
	defer ts.TestEnvironment.Cleanup()

	m.Run()
}

func TestReturnsNotFoundForNonExistentStream(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)

	// Act
	_, err := repo.GetStream(ctx, events.StreamID(createUUID(t)))

	// Assert
	assert.ErrorIs(t, err, events.ErrStreamNotFound)
}

func TestCanCreateStream(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)
	streamToCreate := events.StreamToCreate{
		StreamID:       events.StreamID(createUUID(t)),
		IdempotencyKey: events.IdempotencyKey(createUUID(t)),
	}

	// Act
	stream, err := repo.CreateStream(ctx, streamToCreate)

	// Assert
	assert.Nil(t, err)
	assert.Equal(t, streamToCreate.StreamID, stream.StreamID())
	assert.Equal(t, streamToCreate.IdempotencyKey, stream.IdempotencyKey())
}

func TestCanGetCreatedStream(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)
	streamToCreate := events.StreamToCreate{
		StreamID:       events.StreamID(createUUID(t)),
		IdempotencyKey: events.IdempotencyKey(createUUID(t)),
	}

	// Act
	createdStream, createErr := repo.CreateStream(ctx, streamToCreate)
	stream, getErr := repo.GetStream(ctx, streamToCreate.StreamID)

	// Assert
	assert.Nil(t, createErr)
	assert.Nil(t, getErr)
	assert.Equal(t, createdStream, stream)
}

func TestOnlyCreatesStreamOnceWithSameIdempotencyKey(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)

	successfulStreamID := events.StreamID(createUUID(t))
	failedStreamID := events.StreamID(createUUID(t))
	idempotencyKey := events.IdempotencyKey(createUUID(t))

	// Act
	_, createErr := repo.CreateStream(ctx, events.StreamToCreate{StreamID: successfulStreamID, IdempotencyKey: idempotencyKey})
	_, failedErr := repo.CreateStream(ctx, events.StreamToCreate{StreamID: failedStreamID, IdempotencyKey: idempotencyKey})

	// Assert
	assert.Nil(t, createErr)
	assert.ErrorIs(t, failedErr, events.ErrStreamAlreadyExists)
}

func TestOnlyCreatesStreamOnceWithSameStreamKey(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)

	streamID := events.StreamID(createUUID(t))
	successfulIdempotencyKey := events.IdempotencyKey(createUUID(t))
	failedIdempotencyKey := events.IdempotencyKey(createUUID(t))

	// Act
	_, createErr := repo.CreateStream(ctx, events.StreamToCreate{StreamID: streamID, IdempotencyKey: successfulIdempotencyKey})
	_, failedErr := repo.CreateStream(ctx, events.StreamToCreate{StreamID: streamID, IdempotencyKey: failedIdempotencyKey})

	// Assert
	assert.Nil(t, createErr)
	assert.ErrorIs(t, failedErr, events.ErrStreamAlreadyExists)
}

func TestConcurrentInsertsRespectIdempotency(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)
	idempotencyKey := events.IdempotencyKey(createUUID(t))
	var successfulInserts atomic.Uint32
	var wg sync.WaitGroup

	// Act
	for i := 0; i < 50; i++ {
		wg.Add(1)

		go func() {
			streamID := events.StreamID(createUUID(t))
			_, err := repo.CreateStream(ctx, events.StreamToCreate{StreamID: streamID, IdempotencyKey: idempotencyKey})
			if err == nil {
				successfulInserts.Add(1)
			} else {
				assert.ErrorIs(t, err, events.ErrStreamAlreadyExists)
			}
			wg.Done()
		}()
	}

	// Assert
	wg.Wait()
	assert.Equal(t, uint32(1), successfulInserts.Load())
}

func TestAddingEventWithNonExistingStreamFails(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)

	// Act
	_, err := repo.AddEventToStream(ctx, events.EventToAdd{
		StreamID:        events.StreamID(createUUID(t)),
		IdempotencyKey:  events.IdempotencyKey(createUUID(t)),
		ExpectedVersion: 0,
		Payload:         &testPayload{payload: []byte{}},
	})

	// Assert
	assert.ErrorIs(t, err, events.ErrCouldNotIncrementStreamVersion)
}

func TestCanAddEventToStream(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)
	stream := createTestStream(t, repo)
	payload := []byte("{\"test\" :\"success\"}")
	var successfulInserts atomic.Uint32
	var wg sync.WaitGroup

	// Act
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			_, err := repo.AddEventToStream(ctx, events.EventToAdd{
				StreamID:        stream.StreamID(),
				IdempotencyKey:  events.IdempotencyKey(createUUID(t)),
				ExpectedVersion: stream.LatestVersion(),
				Payload:         &testPayload{payload},
			})
			if err == nil {
				successfulInserts.Add(1)
			} else {
				assert.ErrorIs(t, err, events.ErrCouldNotIncrementStreamVersion)
			}
			wg.Done()
		}()
	}

	// Assert
	wg.Wait()
	assert.Equal(t, uint32(1), successfulInserts.Load())
}

func TestHandlesConcurrentEventInsertionsWithSameExpectedStreamVersion(t *testing.T) {
	// Arrange
	defer ts.TestEnvironment.Reset()
	ctx := context.Background()
	repo := events.NewRepository(ts.TestEnvironment.DB)
	stream := createTestStream(t, repo)
	payload := []byte("{\"test\" :\"success\"}")

	// Act
	event, err := repo.AddEventToStream(ctx, events.EventToAdd{
		StreamID:        stream.StreamID(),
		IdempotencyKey:  events.IdempotencyKey(createUUID(t)),
		ExpectedVersion: stream.LatestVersion(),
		Payload:         &testPayload{payload},
	})

	// Assert
	assert.Nil(t, err)
	assert.JSONEq(t, string(payload), string(event.Payload()))
	assert.Equal(t, stream.LatestVersion()+1, event.Version())
}

type testPayload struct {
	payload []byte
}

func (p *testPayload) SerializePayload() []byte {
	return p.payload
}

func createTestStream(t *testing.T, repo events.EventsRepository) events.Stream {
	stream, err := repo.CreateStream(context.Background(), events.StreamToCreate{
		StreamID:       events.StreamID(createUUID(t)),
		IdempotencyKey: events.IdempotencyKey(createUUID(t)),
	})
	assert.Nil(t, err)
	return stream
}

func createUUID(t *testing.T) uuid.UUID {
	uuid, err := uuid.NewV4()
	assert.Nil(t, err)

	return uuid
}
