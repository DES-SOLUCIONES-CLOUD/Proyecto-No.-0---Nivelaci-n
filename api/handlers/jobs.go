package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"okf-api/db"
	"okf-api/middleware"
	"okf-api/models"
	"okf-api/storage"
)

// JobHandler gestiona el estado y descarga de trabajos.
type JobHandler struct {
	db      *db.DB
	storage *storage.MinIO
}

// NewJobHandler crea un nuevo JobHandler.
func NewJobHandler(database *db.DB, store *storage.MinIO) *JobHandler {
	return &JobHandler{db: database, storage: store}
}

// GetJob maneja GET /api/jobs/:id
// Retorna el estado actual del trabajo. Verifica que pertenezca al usuario autenticado (aislamiento).
func (h *JobHandler) GetJob(c *gin.Context) {
	userID := middleware.GetUserID(c)
	jobID := c.Param("id")

	job, err := h.db.GetJob(jobID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando trabajo"})
		return
	}

	// Si no existe O pertenece a otro usuario, retornar 404 (no revelar información)
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trabajo no encontrado"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// ListJobs maneja GET /api/jobs
// Lista todos los trabajos del usuario autenticado.
func (h *JobHandler) ListJobs(c *gin.Context) {
	userID := middleware.GetUserID(c)

	jobs, err := h.db.ListJobs(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error listando trabajos"})
		return
	}

	// Siempre retornar array (nunca null)
	if jobs == nil {
		jobs = []*models.Job{}
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":  jobs,
		"count": len(jobs),
	})
}

// DownloadBundle maneja GET /api/jobs/:id/download
// Descarga el bundle ZIP del trabajo. Verifica propiedad del usuario (aislamiento).
func (h *JobHandler) DownloadBundle(c *gin.Context) {
	userID := middleware.GetUserID(c)
	jobID := c.Param("id")

	job, err := h.db.GetJob(jobID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error consultando trabajo"})
		return
	}

	// Aislamiento: si no es del usuario, 404 (no revelar existencia)
	if job == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "trabajo no encontrado"})
		return
	}

	// Solo se puede descargar si el bundle está completado y validado
	if job.Status != models.StatusCompleted {
		c.JSON(http.StatusConflict, gin.H{
			"error":  fmt.Sprintf("el bundle no está disponible (estado: %s)", job.Status),
			"status": job.Status,
		})
		return
	}

	if job.BundlePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "bundle no disponible"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Descargar el ZIP del bundle desde MinIO
	obj, err := h.storage.DownloadObject(ctx, h.storage.BucketBundl, job.BundlePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error descargando bundle"})
		return
	}
	defer obj.Close()

	bundleData, err := io.ReadAll(obj)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error leyendo bundle"})
		return
	}

	// Verificar que el ZIP es válido antes de enviarlo
	if _, err := zip.NewReader(bytes.NewReader(bundleData), int64(len(bundleData))); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bundle corrupto"})
		return
	}

	filename := fmt.Sprintf("bundle-%s.zip", jobID[:8])
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Length", fmt.Sprintf("%d", len(bundleData)))
	c.Data(http.StatusOK, "application/zip", bundleData)
}
