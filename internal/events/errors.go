package events

import "errors"

var ErrCouldNotFetchEventStream = errors.New("could not fetch event stream")
var ErrStreamNotFound = errors.New("stream not found")
var ErrFailedToGetStream = errors.New("failed to get stream")
var ErrInvalidStreamId = errors.New("expected UUID stream identifier")
var ErrInvalidStreamVersion = errors.New("expected unsigned 64-bit integer as stream version")
var ErrStreamAlreadyExists = errors.New("this stream already exists")
var ErrFailedToInsertEventToStream = errors.New("could not add event to stream because an unknown error happened")
var ErrCouldNotIncrementStreamVersion = errors.New("could not increment stream version")
var ErrInvalidEventTimestamp = errors.New("invalid event timestamp")
