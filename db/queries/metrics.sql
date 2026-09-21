-- name: NextAggregationBucket
SELECT next_closed_aggregation_bucket();

-- name: AggregateClosedHour
SELECT aggregate_closed_hour($1::timestamptz,$2::uuid);

-- name: PurgeExpiredMetadata
SELECT * FROM purge_expired_metadata($1::uuid);
