package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"okf-api/config"
	"okf-api/db"
	"okf-api/handlers"
	"okf-api/middleware"
	"okf-api/queue"
	"okf-api/storage"
)

func main() {
	// ── Cargar configuración desde variables de entorno ──────────────────────
	cfg := config.Load()
	log.Printf("[API] Iniciando en puerto %s", cfg.Port)

	// ── Inicializar dependencias ─────────────────────────────────────────────
	database, err := db.New(cfg.PostgresURL)
	if err != nil {
		log.Fatalf("[API] Error conectando a PostgreSQL: %v", err)
	}
	defer database.Close()

	store, err := storage.NewMinIO(
		cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey,
		cfg.MinioBucketOrig, cfg.MinioBucketBund, cfg.MinioUseSSL,
	)
	if err != nil {
		log.Fatalf("[API] Error conectando a MinIO: %v", err)
	}

	redisQueue, err := queue.NewRedisQueue(cfg.RedisURL)
	if err != nil {
		log.Fatalf("[API] Error conectando a Redis: %v", err)
	}
	defer redisQueue.Close()

	// ── Configurar router Gin ────────────────────────────────────────────────
	router := gin.Default()

	// CORS — permite peticiones desde el frontend
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Límite de tamaño de petición: 50 MB
	router.MaxMultipartMemory = 50 << 20

	// ── Handlers ─────────────────────────────────────────────────────────────
	authH := handlers.NewAuthHandler(database, cfg.JWTSecret)
	docH := handlers.NewDocumentHandler(database, store, redisQueue)
	jobH := handlers.NewJobHandler(database, store)

	// ── Rutas ────────────────────────────────────────────────────────────────
	api := router.Group("/api")
	{
		// Health check (sin autenticación)
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Autenticación (sin JWT requerido)
		auth := api.Group("/auth")
		{
			auth.POST("/register", authH.Register)
			auth.POST("/login", authH.Login)
		}

		// Rutas protegidas con JWT
		protected := api.Group("/")
		protected.Use(middleware.Auth(cfg.JWTSecret))
		{
			protected.GET("/auth/me", authH.Me)

			// Documentos — carga y procesamiento asíncrono
			protected.POST("/documents", docH.Upload)

			// Trabajos — consulta de estado y descarga
			protected.GET("/jobs", jobH.ListJobs)
			protected.GET("/jobs/:id", jobH.GetJob)
			protected.GET("/jobs/:id/download", jobH.DownloadBundle)
		}
	}

	log.Printf("[API] Servidor listo en :%s", cfg.Port)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("[API] Error iniciando servidor: %v", err)
	}
}
