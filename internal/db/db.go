package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LuisMendezTEC/mediaplatform.git/internal/models"
	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS jobs (
		id            TEXT PRIMARY KEY,
		file_id       TEXT NOT NULL,
		file_path     TEXT NOT NULL,
		operation     TEXT NOT NULL,
		output_path   TEXT,
		status        TEXT NOT NULL DEFAULT 'pending',
		priority      INT  NOT NULL DEFAULT 5,
		worker_id     TEXT,
		progress      INT  NOT NULL DEFAULT 0,
		error_msg     TEXT,
		result_url    TEXT,
		retries       INT  NOT NULL DEFAULT 0,
		max_retries   INT  NOT NULL DEFAULT 3,
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		started_at    TIMESTAMPTZ,
		completed_at  TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS workers (
		id           TEXT PRIMARY KEY,
		hostname     TEXT NOT NULL,
		status       TEXT NOT NULL DEFAULT 'idle',
		active_jobs  INT  NOT NULL DEFAULT 0,
		cpu_percent  FLOAT,
		mem_percent  FLOAT,
		last_seen    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS worker_registry (
		id          TEXT PRIMARY KEY,
		hostname    TEXT NOT NULL,
		status      TEXT NOT NULL DEFAULT 'idle',
		last_seen   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_status   ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_worker   ON jobs(worker_id);
	CREATE INDEX IF NOT EXISTS idx_jobs_priority ON jobs(priority DESC);

	CREATE TABLE IF NOT EXISTS cases (
		id           TEXT PRIMARY KEY,
		name         TEXT NOT NULL,
		description  TEXT,
		status       TEXT NOT NULL DEFAULT 'queued',
		priority     INT  NOT NULL DEFAULT 5,
		risk_score   FLOAT DEFAULT 0,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS findings (
		id           TEXT PRIMARY KEY,
		case_id      TEXT NOT NULL REFERENCES cases(id),
		job_id       TEXT NOT NULL REFERENCES jobs(id),
		worker_type  TEXT NOT NULL,
		category     TEXT NOT NULL,
		confidence   FLOAT NOT NULL,
		risk_level   TEXT NOT NULL DEFAULT 'low',
		evidence     JSONB,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_findings_case  ON findings(case_id);
	CREATE INDEX IF NOT EXISTS idx_findings_risk  ON findings(risk_level);
	CREATE INDEX IF NOT EXISTS idx_cases_status   ON cases(status);
	`)
	return err
}

// InsertJob stores a new job in PostgreSQL.
func InsertJob(db *sql.DB, job *models.Job) error {
	_, err := db.Exec(`
		INSERT INTO jobs (id, file_id, file_path, operation, status, priority, max_retries, created_at, case_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		job.ID, job.FileID, job.FilePath, job.Operation,
		job.Status, job.Priority, job.MaxRetries, job.CreatedAt, job.CaseID,
	)
	return err
}

// GetJob returns a job by ID.
func GetJob(db *sql.DB, id string) (*models.Job, error) {
	row := db.QueryRow(`SELECT id, file_path, operation, status, priority,
		worker_id, progress, error_msg, result_url, retries, max_retries,
		created_at, started_at, completed_at, case_id FROM jobs WHERE id=$1`, id)
	return scanJob(row)
}

// ListJobs returns all jobs, optionally filtered by status.
func ListJobs(db *sql.DB, status string) ([]*models.Job, error) {
	query := `SELECT id, file_path, operation, status, priority,
		worker_id, progress, error_msg, result_url, retries, max_retries,
		created_at, started_at, completed_at, case_id FROM jobs`
	args := []any{}
	if status != "" {
		query += " WHERE status=$1"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 2000"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]*models.Job, 0)
	for rows.Next() {
		j, err := scanJob(rows)
		if err == nil {
			jobs = append(jobs, j)
		}
	}
	return jobs, nil
}

// GetStats returns counts by status for the dashboard.
func GetStats(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`
		SELECT status, COUNT(*) FROM jobs GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := map[string]int{}
	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		stats[status] = count
	}
	return stats, nil
}

// scanJob is a helper to avoid repeating Scan.
func scanJob(row interface {
	Scan(...any) error
}) (*models.Job, error) {
	j := &models.Job{}
	var workerID, errorMsg, resultURL, caseID sql.NullString
	err := row.Scan(
		&j.ID, &j.FilePath, &j.Operation, &j.Status, &j.Priority,
		&workerID, &j.Progress, &errorMsg, &resultURL,
		&j.Retries, &j.MaxRetries, &j.CreatedAt, &j.StartedAt, &j.CompletedAt, &caseID,
	)
	if err != nil {
		return nil, err
	}
	j.WorkerID = workerID.String
	j.ErrorMsg = errorMsg.String
	j.ResultURL = resultURL.String
	j.CaseID = caseID.String
	return j, nil
}

// InsertCase stores a new case in PostgreSQL.
func InsertCase(db *sql.DB, c *models.Case) error {
	_, err := db.Exec(`
		INSERT INTO cases (id, name, description, status, priority, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		c.ID, c.Name, c.Description, c.Status, c.Priority, c.CreatedAt,
	)
	return err
}

// GetCase returns a case by ID.
func GetCase(db *sql.DB, id string) (*models.Case, error) {
	row := db.QueryRow(`SELECT id, name, description, status, priority, risk_score, created_at, completed_at FROM cases WHERE id=$1`, id)
	var c models.Case
	var completedAt sql.NullTime
	err := row.Scan(&c.ID, &c.Name, &c.Description, &c.Status, &c.Priority, &c.RiskScore, &c.CreatedAt, &completedAt)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		c.CompletedAt = &completedAt.Time
	}
	return &c, nil
}

// ListCases returns all cases, optionally filtered by status.
func ListCases(db *sql.DB, status string) ([]*models.Case, error) {
	query := `SELECT id, name, description, status, priority, risk_score, created_at, completed_at FROM cases`
	args := []any{}
	if status != "" {
		query += " WHERE status=$1"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT 500"
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cases := make([]*models.Case, 0)
	for rows.Next() {
		var c models.Case
		var completedAt sql.NullTime
		err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Status, &c.Priority, &c.RiskScore, &c.CreatedAt, &completedAt)
		if err == nil {
			if completedAt.Valid {
				c.CompletedAt = &completedAt.Time
			}
			cases = append(cases, &c)
		}
	}
	return cases, nil
}

// InsertFinding stores a new finding in PostgreSQL.
func InsertFinding(db *sql.DB, f *models.Finding) error {
	evidence, _ := json.Marshal(f.Evidence)
	_, err := db.Exec(`
		INSERT INTO findings (id, case_id, job_id, worker_type, category, confidence, risk_level, evidence, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		f.ID, f.CaseID, f.JobID, f.WorkerType, f.Category, f.Confidence, f.RiskLevel, evidence, f.CreatedAt,
	)
	return err
}

// GetFindingsByCase returns all findings for a case.
func GetFindingsByCase(db *sql.DB, caseID string) ([]*models.Finding, error) {
	rows, err := db.Query(`SELECT id, case_id, job_id, worker_type, category, confidence, risk_level, evidence, created_at FROM findings WHERE case_id=$1 ORDER BY created_at DESC`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	findings := make([]*models.Finding, 0)
	for rows.Next() {
		var f models.Finding
		var evidenceJSON []byte
		err := rows.Scan(&f.ID, &f.CaseID, &f.JobID, &f.WorkerType, &f.Category, &f.Confidence, &f.RiskLevel, &evidenceJSON, &f.CreatedAt)
		if err == nil {
			json.Unmarshal(evidenceJSON, &f.Evidence)
			findings = append(findings, &f)
		}
	}
	return findings, nil
}

// UpdateCaseStatus updates a case's status and optionally its risk_score and completed_at.
func UpdateCaseStatus(db *sql.DB, caseID string, status string, riskScore *float64, completedAt *time.Time) error {
	if completedAt != nil && riskScore != nil {
		_, err := db.Exec(`UPDATE cases SET status=$1, risk_score=$2, completed_at=$3 WHERE id=$4`,
			status, *riskScore, *completedAt, caseID)
		return err
	}
	if riskScore != nil {
		_, err := db.Exec(`UPDATE cases SET status=$1, risk_score=$2 WHERE id=$3`,
			status, *riskScore, caseID)
		return err
	}
	_, err := db.Exec(`UPDATE cases SET status=$1 WHERE id=$2`, status, caseID)
	return err
}
