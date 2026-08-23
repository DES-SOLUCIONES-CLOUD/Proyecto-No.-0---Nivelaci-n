package models

import "time"

// User representa un usuario registrado en la plataforma.
type User struct {
	ID           string    `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// JobStatus representa el estado posible de un trabajo de conversión.
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusCanceled   JobStatus = "canceled"
)

// Job representa un trabajo de conversión documental.
type Job struct {
	ID               string    `json:"id" db:"id"`
	ParentJobID      *string   `json:"parent_job_id,omitempty" db:"parent_job_id"`
	UserID           string    `json:"user_id" db:"user_id"`
	Filename         string    `json:"filename" db:"filename"`
	OriginalFilename string    `json:"original_filename" db:"original_filename"`
	Format           string    `json:"format" db:"format"`
	Status           JobStatus `json:"status" db:"status"`
	ErrorMessage     *string   `json:"error_message,omitempty" db:"error_message"`
	OriginalPath     string    `json:"-" db:"original_path"`
	BundlePath       string    `json:"-" db:"bundle_path"`
	BundleSize       int64     `json:"bundle_size" db:"bundle_size"`
	ConceptCount     int       `json:"concept_count" db:"concept_count"`
	// ValidationStatus: "valid" | "valid_with_warnings" | "invalid" (vacío si el job aún no completó).
	ValidationStatus   string     `json:"validation_status,omitempty" db:"validation_status"`
	ValidationWarnings []string   `json:"validation_warnings,omitempty" db:"validation_warnings"`
	CreatedAt          time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at" db:"updated_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty" db:"completed_at"`
}

// JobMessage es el mensaje publicado en Redis para el worker.
type JobMessage struct {
	JobID        string `json:"job_id"`
	UserID       string `json:"user_id"`
	OriginalPath string `json:"original_path"`
	Format       string `json:"format"`
	Filename     string `json:"filename"`
}

// RegisterRequest es el body de la petición de registro.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest es el body de la petición de login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse es la respuesta de login/registro con el token JWT.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// JobResponse es la respuesta al subir un documento.
type JobResponse struct {
	JobID   string    `json:"job_id"`
	Status  JobStatus `json:"status"`
	Message string    `json:"message"`
}
