CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS room_recommendation_review_identity_uidx ON room_recommendation_review (room_id, memory_revision_id, recommendation_key);
