package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	creator "creator-pipeline/internal/eino"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func EnsureMySQLSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS creation_tasks (
  id VARCHAR(64) PRIMARY KEY,
  user_id VARCHAR(128) NOT NULL,
  idempotency_key VARCHAR(191) NULL,
  prompt TEXT NOT NULL,
  plan_json JSON NOT NULL,
  status VARCHAR(32) NOT NULL,
  attempt INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 2,
  error_code VARCHAR(128) NULL,
  error_message TEXT NULL,
  result_url TEXT NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  started_at DATETIME(6) NULL,
  finished_at DATETIME(6) NULL,
  deadline_at DATETIME(6) NULL,
  UNIQUE KEY uk_creation_tasks_idem (user_id, idempotency_key),
  KEY idx_creation_tasks_status_updated (status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
`)
	return err
}

func (r *MySQLRepository) Create(ctx context.Context, t *Task) error {
	planJSON, err := json.Marshal(t.Plan)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO creation_tasks (
  id, user_id, idempotency_key, prompt, plan_json, status, attempt, max_retries,
  error_code, error_message, result_url, created_at, updated_at, started_at, finished_at, deadline_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.UserID, nullableString(t.IdempotencyKey), t.Prompt, planJSON, t.Status, t.Attempt, t.MaxRetries,
		nullableString(t.ErrorCode), nullableString(t.ErrorMessage), nullableString(t.ResultURL),
		t.CreatedAt, t.UpdatedAt, t.StartedAt, t.FinishedAt, t.DeadlineAt,
	)
	return err
}

func (r *MySQLRepository) Get(ctx context.Context, id string) (*Task, error) {
	row := r.db.QueryRowContext(ctx, selectTaskSQL()+` WHERE id = ?`, id)
	return scanTask(row)
}

func (r *MySQLRepository) Update(ctx context.Context, id string, mutate func(*Task) error) (*Task, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	t, err := scanTask(tx.QueryRowContext(ctx, selectTaskSQL()+` WHERE id = ? FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if err := mutate(t); err != nil {
		return nil, err
	}

	planJSON, err := json.Marshal(t.Plan)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE creation_tasks SET
  user_id = ?,
  idempotency_key = ?,
  prompt = ?,
  plan_json = ?,
  status = ?,
  attempt = ?,
  max_retries = ?,
  error_code = ?,
  error_message = ?,
  result_url = ?,
  created_at = ?,
  updated_at = ?,
  started_at = ?,
  finished_at = ?,
  deadline_at = ?
WHERE id = ?`,
		t.UserID, nullableString(t.IdempotencyKey), t.Prompt, planJSON, t.Status, t.Attempt, t.MaxRetries,
		nullableString(t.ErrorCode), nullableString(t.ErrorMessage), nullableString(t.ResultURL),
		t.CreatedAt, t.UpdatedAt, t.StartedAt, t.FinishedAt, t.DeadlineAt, t.ID,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return t, nil
}

func selectTaskSQL() string {
	return `SELECT id, user_id, COALESCE(idempotency_key, ''), prompt, plan_json, status, attempt, max_retries,
COALESCE(error_code, ''), COALESCE(error_message, ''), COALESCE(result_url, ''),
created_at, updated_at, started_at, finished_at, deadline_at FROM creation_tasks`
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(row scanner) (*Task, error) {
	var t Task
	var planJSON []byte
	var status string
	var startedAt, finishedAt, deadlineAt sql.NullTime
	if err := row.Scan(
		&t.ID, &t.UserID, &t.IdempotencyKey, &t.Prompt, &planJSON, &status, &t.Attempt, &t.MaxRetries,
		&t.ErrorCode, &t.ErrorMessage, &t.ResultURL, &t.CreatedAt, &t.UpdatedAt, &startedAt, &finishedAt, &deadlineAt,
	); err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}
	var plan creator.CreationPlan
	if err := json.Unmarshal(planJSON, &plan); err != nil {
		return nil, err
	}
	t.Plan = plan
	t.PlanJSON = planJSON
	t.Status = Status(status)
	if startedAt.Valid {
		t.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		t.FinishedAt = &finishedAt.Time
	}
	if deadlineAt.Valid {
		t.DeadlineAt = &deadlineAt.Time
	}
	return &t, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
