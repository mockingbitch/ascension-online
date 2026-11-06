-- name: IncrementDailyTask :exec
INSERT INTO daily_task_progress (character_id, game_day, task_id, progress)
VALUES (@character_id, @game_day, @task_id, LEAST(@delta::int, @target::int))
ON CONFLICT (character_id, game_day, task_id) DO UPDATE
SET progress = LEAST(daily_task_progress.progress + @delta::int, @target::int), updated_at = now();

-- name: ListDailyTasks :many
SELECT * FROM daily_task_progress WHERE character_id = $1 AND game_day = $2;

-- name: GetDailyTaskForUpdate :one
SELECT * FROM daily_task_progress WHERE character_id = $1 AND game_day = $2 AND task_id = $3 FOR UPDATE;

-- name: MarkDailyTaskClaimed :exec
UPDATE daily_task_progress SET claimed_at = $4, updated_at = now()
WHERE character_id = $1 AND game_day = $2 AND task_id = $3;
