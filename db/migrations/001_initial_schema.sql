create table streams(
  stream_id uuid primary key,
  idempotency_key uuid not null unique,
  latest_version bigint not null check (latest_version >= 0)
);

create table events(
  event_id bigint primary key generated always as identity,
  stream_id uuid not null references streams(stream_id),
  idempotency_key uuid not null unique,
  recorded_at timestamptz not null default timezone('utc', now()),
  version bigint not null check (version >= 0),
  payload jsonb not null,
  constraint uq_stream_version unique(stream_id, version)
);

---- create above / drop below ----
drop table if exists events;

drop table if exists streams;