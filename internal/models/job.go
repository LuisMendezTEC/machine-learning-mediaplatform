package models

import "time"

type JobStatus string

const (
	StatusPending   JobStatus = "pending"
	StatusAssigned  JobStatus = "assigned"
	StatusRunning   JobStatus = "running"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
)

type Operation string

const (
	OpConvert      Operation = "convert"
	OpExtractAudio Operation = "extract_audio"
	OpThumbnail    Operation = "thumbnail"
	OpConvertAudio Operation = "convert_audio"
	// ML Analysis operations (new)
	OpAnalyzeText  Operation = "analyze_text"
	OpAnalyzeImage Operation = "analyze_image"
	OpAnalyzeAudio Operation = "analyze_audio"
)

type Job struct {
	ID          string     `json:"id"`
	FileID      string     `json:"file_id"`
	FilePath    string     `json:"file_path"`
	Operation   Operation  `json:"operation"`
	OutputPath  string     `json:"output_path"`
	Status      JobStatus  `json:"status"`
	Priority    int        `json:"priority"`
	WorkerID    string     `json:"worker_id"`
	Progress    int        `json:"progress"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
	ResultURL   string     `json:"result_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Retries     int        `json:"retries"`
	MaxRetries  int        `json:"max_retries"`
	CaseID      string     `json:"case_id,omitempty"` // NEW: link to parent case
}

type Case struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Status      string     `json:"status"` // queued, running, completed, failed
	Priority    int        `json:"priority"`
	RiskScore   float64    `json:"risk_score"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Finding struct {
	ID         string                 `json:"id"`
	CaseID     string                 `json:"case_id"`
	JobID      string                 `json:"job_id"`
	WorkerType string                 `json:"worker_type"` // text, image, audio
	Category   string                 `json:"category"`    // keyword, sentiment, violence, etc
	Confidence float64                `json:"confidence"`  // 0.0 - 1.0
	RiskLevel  string                 `json:"risk_level"`  // low, medium, high, critical
	Evidence   map[string]interface{} `json:"evidence"`    // text_fragment, timestamp, bounding_box, etc
	CreatedAt  time.Time              `json:"created_at"`
}

type WorkerInfo struct {
	ID         string    `json:"id"`
	Hostname   string    `json:"hostname"`
	Status     string    `json:"status"`
	ActiveJobs int       `json:"active_jobs"`
	CPUPercent float64   `json:"cpu_percent"`
	MemPercent float64   `json:"mem_percent"`
	LastSeen   time.Time `json:"last_seen"`
}

type JobEvent struct {
	JobID    string    `json:"job_id"`
	Status   JobStatus `json:"status"`
	Progress int       `json:"progress"`
	WorkerID string    `json:"worker_id"`
	Message  string    `json:"message,omitempty"`
}
