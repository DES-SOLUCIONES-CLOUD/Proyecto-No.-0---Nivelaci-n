package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"okf-worker/bundle"
	"okf-worker/converter"
	"okf-worker/queue"
	"okf-worker/storage"
)

// ─── Configuración ──────────────────────────────────────────────────────────

type config struct {
	PostgresURL string
	RedisURL    string
	MinioEP     string
	MinioAK     string
	MiniSK      string
	BucketOrig  string
	BucketBundl string
	MinioSSL    bool
	WorkerName  string
}

func loadConfig() *config {
	hostname, _ := os.Hostname()
	return &config{
		PostgresURL: getEnv("POSTGRES_URL", "postgres://okf:okfpassword@localhost:5432/okf?sslmode=disable"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379"),
		MinioEP:     getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAK:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MiniSK:      getEnv("MINIO_SECRET_KEY", "minioadmin123"),
		BucketOrig:  getEnv("MINIO_BUCKET_ORIGINALS", "originals"),
		BucketBundl: getEnv("MINIO_BUCKET_BUNDLES", "bundles"),
		MinioSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
		WorkerName:  "worker-" + hostname,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ─── Main ────────────────────────────────────────────────────────────────────

func main() {
	cfg := loadConfig()
	log.Printf("[Worker] Iniciando como '%s'", cfg.WorkerName)

	// Inicializar dependencias
	db, err := connectDB(cfg.PostgresURL)
	if err != nil {
		log.Fatalf("[Worker] Error conectando a PostgreSQL: %v", err)
	}
	defer db.Close()

	store, err := storage.NewMinIO(cfg.MinioEP, cfg.MinioAK, cfg.MiniSK, cfg.BucketOrig, cfg.BucketBundl, cfg.MinioSSL)
	if err != nil {
		log.Fatalf("[Worker] Error conectando a MinIO: %v", err)
	}

	consumer, err := queue.NewRedisConsumer(cfg.RedisURL, cfg.WorkerName)
	if err != nil {
		log.Fatalf("[Worker] Error conectando a Redis: %v", err)
	}
	defer consumer.Close()

	// Contexto con cancelación para shutdown graceful
	ctx, cancel := context.WithCancel(context.Background())

	// Capturar señales de parada
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[Worker] Señal de parada recibida, finalizando...")
		cancel()
	}()

	// Procesar mensajes
	log.Println("[Worker] Esperando trabajos...")
	messages := consumer.ReadMessages(ctx)

	for msg := range messages {
		log.Printf("[Worker] Procesando job %s", msg.JobID)
		processJob(ctx, db, store, consumer, msg)
	}

	log.Println("[Worker] Terminado")
}

// ─── Procesamiento de Jobs ───────────────────────────────────────────────────

func processJob(ctx context.Context, db *sql.DB, store *storage.MinIO, consumer *queue.RedisConsumer, msg queue.JobMessage) {
	// IDEMPOTENCIA: verificar si ya fue procesado
	first, err := markProcessed(db, msg.JobID)
	if err != nil {
		log.Printf("[Worker] Error verificando idempotencia para job %s: %v", msg.JobID, err)
		// No hacer ACK — reintentar después
		return
	}
	if !first {
		log.Printf("[Worker] Job %s ya fue procesado anteriormente, descartando duplicado", msg.JobID)
		_ = consumer.Acknowledge(ctx, msg.MsgID)
		return
	}

	// VERIFICAR CANCELACIÓN
	canceled, err := isCanceled(db, msg.JobID)
	if err == nil && canceled {
		log.Printf("[Worker] Job %s fue cancelado por el usuario, abortando procesamiento", msg.JobID)
		_ = consumer.Acknowledge(ctx, msg.MsgID)
		return
	}

	// Verificar reintentos máximos (Política de reintentos)
	const maxRetries = 3
	if consumer.GetDeliveryCount(ctx, msg.MsgID) > maxRetries {
		log.Printf("[Worker] Job %s excedió %d reintentos, marcando como fallido", msg.JobID, maxRetries)
		_ = updateStatus(db, msg.JobID, "failed", "excedidos reintentos máximos permitidos")
		_ = consumer.Acknowledge(ctx, msg.MsgID)
		return
	}

	// Marcar como "processing"
	if err := updateStatus(db, msg.JobID, "processing", ""); err != nil {
		log.Printf("[Worker] Error actualizando estado a processing: %v", err)
	}

	// Ejecutar conversión con timeout de 5 minutos
	jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	bundleResult, err := convertDocument(jobCtx, store, msg)
	if err != nil {
		log.Printf("[Worker] Error convirtiendo job %s: %v", msg.JobID, err)
		_ = updateStatus(db, msg.JobID, "failed", err.Error())
		_ = consumer.Acknowledge(ctx, msg.MsgID)
		return
	}

	// Validar bundle antes de publicar
	validation, err := bundle.Validate(bundleResult.ZipData)
	if err != nil {
		log.Printf("[Worker] Error validando bundle para job %s: %v", msg.JobID, err)
		_ = updateStatus(db, msg.JobID, "failed", "error validando bundle: "+err.Error())
		_ = consumer.Acknowledge(ctx, msg.MsgID)
		return
	}

	// Si el bundle no pasa validación, NO publicar
	if !validation.Valid {
		errMsg := "bundle inválido: " + strings.Join(validation.Errors, "; ")
		log.Printf("[Worker] Bundle inválido para job %s: %s", msg.JobID, errMsg)
		_ = updateStatus(db, msg.JobID, "failed", errMsg)
		_ = consumer.Acknowledge(ctx, msg.MsgID)
		return
	}

	if validation.Status == bundle.StatusValidWarning {
		log.Printf("[Worker] Bundle válido con advertencias para job %s: %s", msg.JobID, strings.Join(validation.Warnings, "; "))
	}

	// Subir el bundle final a MinIO
	bundlePath := fmt.Sprintf("%s/%s/bundle.zip", msg.UserID, msg.JobID)
	if err := store.UploadBundle(jobCtx, bundlePath, bundleResult.ZipData); err != nil {
		log.Printf("[Worker] Error subiendo bundle para job %s: %v", msg.JobID, err)
		_ = updateStatus(db, msg.JobID, "failed", "error subiendo bundle: "+err.Error())
		_ = consumer.Acknowledge(ctx, msg.MsgID)
		return
	}

	// Actualizar metadatos del bundle en PostgreSQL
	if err := updateBundle(db, msg.JobID, bundlePath, int64(len(bundleResult.ZipData)), bundleResult.ConceptCount); err != nil {
		log.Printf("[Worker] Error actualizando metadatos bundle: %v", err)
	}

	// Marcar como completado
	if err := updateStatus(db, msg.JobID, "completed", ""); err != nil {
		log.Printf("[Worker] Error actualizando estado a completed: %v", err)
	}

	log.Printf("[Worker] Job %s completado exitosamente (%d conceptos)", msg.JobID, bundleResult.ConceptCount)

	// Confirmar mensaje — no se reprocesará
	_ = consumer.Acknowledge(ctx, msg.MsgID)
}

// convertDocument descarga el original y genera el bundle OKF.
func convertDocument(ctx context.Context, store *storage.MinIO, msg queue.JobMessage) (*bundle.GenerationResult, error) {
	// Descargar documento original desde MinIO
	data, err := store.DownloadOriginal(ctx, msg.OriginalPath)
	if err != nil {
		return nil, fmt.Errorf("error descargando documento: %w", err)
	}

	// Parsear según el formato detectado
	content := string(data)
	var units []converter.Unit

	switch msg.Format {
	case "markdown":
		units = converter.ParseMarkdown(content)
	case "html":
		units = converter.ParseHTML(content)
	case "plaintext":
		units = converter.ParsePlainText(content)
	case "pdf":
		pdfUnits, pdfErr := converter.ParsePDF(data)
		if pdfErr != nil {
			return nil, fmt.Errorf("error procesando PDF: %w", pdfErr)
		}
		units = pdfUnits
	case "docx":
		units = converter.ParseDOCX(data)
	default:
		// Fallback: tratar como texto plano
		units = converter.ParsePlainText(content)
	}

	log.Printf("[Worker] Documento '%s' dividido en %d unidades", msg.Filename, len(units))

	// Generar bundle OKF
	result, err := bundle.Generate(msg.JobID, msg.Filename, msg.Format, units)
	if err != nil {
		return nil, fmt.Errorf("error generando bundle: %w", err)
	}

	return result, nil
}

// ─── Acceso a PostgreSQL ─────────────────────────────────────────────────────

func connectDB(url string) (*sql.DB, error) {
	var db *sql.DB
	var err error
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", url)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("[Worker DB] Intento %d/10: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	if err != nil {
		return nil, err
	}
	log.Println("[Worker DB] Conexión a PostgreSQL establecida")
	return db, nil
}

func markProcessed(db *sql.DB, jobID string) (bool, error) {
	result, err := db.Exec(
		`INSERT INTO job_idempotency (job_id) VALUES ($1) ON CONFLICT (job_id) DO NOTHING`,
		jobID,
	)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func updateStatus(db *sql.DB, jobID, status, errMsg string) error {
	var err error
	if status == "completed" || status == "failed" {
		_, err = db.Exec(
			`UPDATE jobs SET status = $2, error_message = NULLIF($3,''), completed_at = NOW() WHERE id = $1`,
			jobID, status, errMsg,
		)
	} else {
		_, err = db.Exec(
			`UPDATE jobs SET status = $2, error_message = NULLIF($3,'') WHERE id = $1`,
			jobID, status, errMsg,
		)
	}
	return err
}

func updateBundle(db *sql.DB, jobID, bundlePath string, size int64, conceptCount int) error {
	_, err := db.Exec(
		`UPDATE jobs SET bundle_path = $2, bundle_size = $3, concept_count = $4 WHERE id = $1`,
		jobID, bundlePath, size, conceptCount,
	)
	return err
}

func isCanceled(db *sql.DB, jobID string) (bool, error) {
	var status string
	err := db.QueryRow(`SELECT status FROM jobs WHERE id = $1`, jobID).Scan(&status)
	if err != nil {
		return false, err
	}
	return status == "canceled", nil
}
