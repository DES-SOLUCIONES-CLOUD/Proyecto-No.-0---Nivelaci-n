package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"okf-api/db"
	"okf-api/middleware"
	"okf-api/models"
	"okf-api/queue"
	"okf-api/storage"
)

// DocumentHandler gestiona la carga de documentos.
type DocumentHandler struct {
	db      *db.DB
	storage *storage.MinIO
	queue   *queue.RedisQueue
}

// NewDocumentHandler crea un nuevo DocumentHandler.
func NewDocumentHandler(database *db.DB, store *storage.MinIO, q *queue.RedisQueue) *DocumentHandler {
	return &DocumentHandler{db: database, storage: store, queue: q}
}

// Upload maneja POST /api/documents
// Recibe el archivo, lo guarda en MinIO, crea el Job en DB, publica en Redis y retorna INMEDIATAMENTE.
// La conversión NO ocurre aquí — es completamente asíncrona.
func (h *DocumentHandler) Upload(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Leer archivo del form multipart (máx 50 MB)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "se requiere un archivo en el campo 'file'"})
		return
	}
	defer file.Close()

	// Validar tamaño (50 MB máximo)
	const maxSize = 50 << 20
	if header.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "el archivo supera el límite de 50 MB"})
		return
	}

	// Detectar formato del archivo
	format := detectFormat(header.Filename)
	if format == "unknown" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "formato no soportado. Usa: .md, .html, .htm, .txt",
		})
		return
	}

	// Leer contenido en memoria para detectar tipo MIME real
	content, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error leyendo archivo"})
		return
	}

	// Generar IDs únicos para el job y la ruta en MinIO
	jobID := uuid.New().String()
	safeFilename := sanitizeFilename(header.Filename)
	objectPath := fmt.Sprintf("%s/%s/%s", userID, jobID, safeFilename)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Subir documento original a MinIO
	if err := h.storage.UploadObject(ctx, h.storage.BucketOrig, objectPath,
		bytes.NewReader(content), int64(len(content)), contentTypeFor(format)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error almacenando documento"})
		return
	}

	// Crear registro del Job en PostgreSQL
	job := &models.Job{
		ID:               jobID,
		UserID:           userID,
		Filename:         safeFilename,
		OriginalFilename: header.Filename,
		Format:           format,
		Status:           models.StatusPending,
		OriginalPath:     objectPath,
	}

	if err := h.db.CreateJob(job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error registrando trabajo"})
		return
	}

	// Publicar mensaje en Redis Stream — la API retorna ANTES de que el worker procese
	msg := &models.JobMessage{
		JobID:        jobID,
		UserID:       userID,
		OriginalPath: objectPath,
		Format:       format,
		Filename:     safeFilename,
	}

	if err := h.queue.PublishJob(ctx, msg); err != nil {
		// Aunque falle la publicación, el job está en DB — se podría reintentar
		_ = h.db.UpdateJobStatus(jobID, models.StatusFailed, "error publicando en cola")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error encolando trabajo"})
		return
	}

	// Respuesta inmediata — la conversión aún no comenzó
	c.JSON(http.StatusAccepted, models.JobResponse{
		JobID:   jobID,
		Status:  models.StatusPending,
		Message: "documento recibido, procesamiento iniciado en segundo plano",
	})
}

// detectFormat detecta el formato del archivo por extensión.
func detectFormat(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md", ".markdown":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".txt":
		return "plaintext"
	default:
		return "unknown"
	}
}

// sanitizeFilename limpia el nombre del archivo para uso seguro.
func sanitizeFilename(filename string) string {
	// Reemplazar caracteres peligrosos
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", "..", "_")
	clean := replacer.Replace(filepath.Base(filename))
	if clean == "" {
		clean = "document"
	}
	return clean
}

// contentTypeFor retorna el content type MIME para un formato.
func contentTypeFor(format string) string {
	switch format {
	case "markdown":
		return "text/markdown"
	case "html":
		return "text/html"
	default:
		return "text/plain"
	}
}
