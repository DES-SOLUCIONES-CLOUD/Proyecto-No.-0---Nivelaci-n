package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq" // driver PostgreSQL
	"okf-api/models"
)

// DB envuelve la conexión a PostgreSQL y expone los métodos de acceso a datos.
type DB struct {
	conn *sql.DB
}

// New crea una nueva conexión a PostgreSQL con reintentos.
func New(postgresURL string) (*DB, error) {
	var conn *sql.DB
	var err error

	// Reintentar conexión hasta 10 veces con backoff
	for i := 0; i < 10; i++ {
		conn, err = sql.Open("postgres", postgresURL)
		if err == nil {
			err = conn.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("[DB] Intento %d/10 fallido: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("no se pudo conectar a PostgreSQL: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	log.Println("[DB] Conexión a PostgreSQL establecida")
	return &DB{conn: conn}, nil
}

// Close cierra la conexión.
func (d *DB) Close() error {
	return d.conn.Close()
}

// ─── Usuarios ────────────────────────────────────────────────────────────────

// CreateUser inserta un nuevo usuario en la base de datos.
func (d *DB) CreateUser(u *models.User) error {
	query := `
		INSERT INTO users (id, username, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at, updated_at`
	return d.conn.QueryRow(query, u.ID, u.Username, u.Email, u.PasswordHash).
		Scan(&u.CreatedAt, &u.UpdatedAt)
}

// GetUserByEmail busca un usuario por email.
func (d *DB) GetUserByEmail(email string) (*models.User, error) {
	u := &models.User{}
	query := `SELECT id, username, email, password_hash, created_at, updated_at
	          FROM users WHERE email = $1`
	err := d.conn.QueryRow(query, email).
		Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// GetUserByUsername busca un usuario por username.
func (d *DB) GetUserByUsername(username string) (*models.User, error) {
	u := &models.User{}
	query := `SELECT id, username, email, password_hash, created_at, updated_at
	          FROM users WHERE username = $1`
	err := d.conn.QueryRow(query, username).
		Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// ─── Jobs ────────────────────────────────────────────────────────────────────

// CreateJob inserta un nuevo trabajo de conversión.
func (d *DB) CreateJob(j *models.Job) error {
	query := `
		INSERT INTO jobs (id, parent_job_id, user_id, filename, original_filename, format, status, original_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at`
	return d.conn.QueryRow(query,
		j.ID, j.ParentJobID, j.UserID, j.Filename, j.OriginalFilename, j.Format, j.Status, j.OriginalPath,
	).Scan(&j.CreatedAt, &j.UpdatedAt)
}

// GetJob obtiene un trabajo por ID. Verifica que pertenezca al usuario (aislamiento).
func (d *DB) GetJob(jobID, userID string) (*models.Job, error) {
	j := &models.Job{}
	var errMsg sql.NullString
	var completedAt sql.NullTime
	var bundlePath sql.NullString
	query := `
		SELECT id, parent_job_id, user_id, filename, original_filename, format, status,
		       error_message, original_path, bundle_path, bundle_size, concept_count,
		       created_at, updated_at, completed_at
		FROM jobs WHERE id = $1 AND user_id = $2`
	var parentJobID sql.NullString
	err := d.conn.QueryRow(query, jobID, userID).Scan(
		&j.ID, &parentJobID, &j.UserID, &j.Filename, &j.OriginalFilename, &j.Format, &j.Status,
		&errMsg, &j.OriginalPath, &bundlePath, &j.BundleSize, &j.ConceptCount,
		&j.CreatedAt, &j.UpdatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if errMsg.Valid {
		j.ErrorMessage = &errMsg.String
	}
	if bundlePath.Valid {
		j.BundlePath = bundlePath.String
	}
	if parentJobID.Valid {
		j.ParentJobID = &parentJobID.String
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return j, nil
}

// ListJobs lista todos los trabajos de un usuario.
func (d *DB) ListJobs(userID string) ([]*models.Job, error) {
	query := `
		SELECT id, parent_job_id, user_id, filename, original_filename, format, status,
		       error_message, bundle_size, concept_count, created_at, updated_at, completed_at
		FROM jobs WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := d.conn.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		j := &models.Job{}
		var errMsg sql.NullString
		var completedAt sql.NullTime
		var parentJobID sql.NullString
		err := rows.Scan(
			&j.ID, &parentJobID, &j.UserID, &j.Filename, &j.OriginalFilename, &j.Format, &j.Status,
			&errMsg, &j.BundleSize, &j.ConceptCount, &j.CreatedAt, &j.UpdatedAt, &completedAt,
		)
		if err != nil {
			return nil, err
		}
		if errMsg.Valid {
			j.ErrorMessage = &errMsg.String
		}
		if parentJobID.Valid {
			j.ParentJobID = &parentJobID.String
		}
		if completedAt.Valid {
			j.CompletedAt = &completedAt.Time
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdateJobStatus actualiza el estado de un trabajo.
func (d *DB) UpdateJobStatus(jobID string, status models.JobStatus, errMsg string) error {
	var query string
	if status == models.StatusCompleted || status == models.StatusFailed {
		query = `UPDATE jobs SET status = $2, error_message = NULLIF($3,''), 
		         completed_at = NOW() WHERE id = $1`
	} else {
		query = `UPDATE jobs SET status = $2, error_message = NULLIF($3,'') WHERE id = $1`
	}
	_, err := d.conn.Exec(query, jobID, string(status), errMsg)
	return err
}

// UpdateJobBundle actualiza los metadatos del bundle generado.
func (d *DB) UpdateJobBundle(jobID, bundlePath string, bundleSize int64, conceptCount int) error {
	query := `UPDATE jobs SET bundle_path = $2, bundle_size = $3, concept_count = $4 WHERE id = $1`
	_, err := d.conn.Exec(query, jobID, bundlePath, bundleSize, conceptCount)
	return err
}

// GetMetrics retorna el conteo total de jobs agrupados por estado
func (d *DB) GetMetrics() (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM jobs GROUP BY status`
	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metrics := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		metrics[status] = count
	}
	return metrics, nil
}

// ─── Idempotencia ────────────────────────────────────────────────────────────

// MarkJobProcessed registra que un trabajo ya fue procesado (evita duplicados).
// Devuelve true si fue el primero en registrarlo, false si ya existía.
func (d *DB) MarkJobProcessed(jobID string) (bool, error) {
	query := `INSERT INTO job_idempotency (job_id) VALUES ($1) ON CONFLICT (job_id) DO NOTHING`
	result, err := d.conn.Exec(query, jobID)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}
