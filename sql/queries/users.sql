-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
	$1,
	$2,
	$3,
	$4,
	$5
)
RETURNING *;

-- name: DeleteUsers :exec
DELETE FROM users;

-- name: GetUser :one
SELECT * FROM users WHERE email = $1;

-- name: UpdatePasswordEml :one
UPDATE users
SET hashed_password = $1, email = $2
WHERE id = $3
RETURNING *; 