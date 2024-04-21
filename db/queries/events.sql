-- name: GetEventsForStream :many
select *
from events
where stream_id = @stream_id
order by version asc;

-- name: CreateStream :one
insert into streams (stream_id, latest_version, idempotency_key)
values (@stream_id, 0, @idempotency_key)
returning *;

-- name: GetStream :one
select *
from streams
where stream_id = @stream_id;

-- name: IncrementStreamVersion :one
update streams
set latest_version = latest_version + 1
where stream_id = @stream_id
    and latest_version = @expected_version
returning *;

-- name: AddEventToStream :one
insert into events (stream_id, idempotency_key, version, payload)
values (@stream_id, @idempotency_key, @version, @payload)
returning *;