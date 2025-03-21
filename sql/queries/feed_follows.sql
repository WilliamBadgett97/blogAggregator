-- name: CreateFeedFollow :many
WITH inserted_feed_follow AS (
    INSERT INTO feed_follows(id, created_at, updated_at, user_id, feed_id)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, created_at, updated_at, user_id, feed_id
)
SELECT 
    inserted_feed_follow.id, 
    inserted_feed_follow.created_at, 
    inserted_feed_follow.updated_at, 
    inserted_feed_follow.user_id, 
    inserted_feed_follow.feed_id,
    feeds.name AS feed_name,
    users.name AS user_name
FROM inserted_feed_follow
INNER JOIN users
    ON users.id = inserted_feed_follow.user_id
INNER JOIN feeds
    ON feeds.id = inserted_feed_follow.feed_id;
    

-- name: GetFeedFollowsForUser :many
SELECT 
    users.name AS user_name,
    feeds.name AS feed_name,
    feed_follows.*
FROM feed_follows
INNER JOIN users
    ON users.id = feed_follows.user_id
INNER JOIN feeds
    ON feeds.id = feed_follows.feed_id
WHERE users.id = $1;

-- Add a new SQL query to delete a feed follow record by user and feed url combination

-- name: DeleteFeedFollowsByUserAndUrl :exec
DELETE FROM feed_follows
WHERE user_id = $1 AND feed_id = $2;