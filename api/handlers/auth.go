package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"okf-api/db"
	"okf-api/middleware"
	"okf-api/models"
)

// AuthHandler gestiona los endpoints de autenticación.
type AuthHandler struct {
	db        *db.DB
	jwtSecret string
}

// NewAuthHandler crea un nuevo AuthHandler.
func NewAuthHandler(database *db.DB, jwtSecret string) *AuthHandler {
	return &AuthHandler{db: database, jwtSecret: jwtSecret}
}

// Register maneja POST /api/auth/register
// Crea un nuevo usuario con la contraseña hasheada con bcrypt.
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verificar contraseña
	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "la contraseña debe tener al menos 8 caracteres"})
		return
	}

	// Verificar username único
	existing, err := h.db.GetUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "el nombre de usuario ya está en uso"})
		return
	}

	// Verificar email único
	existing, err = h.db.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno"})
		return
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "el email ya está registrado"})
		return
	}

	// Hashear contraseña
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error procesando contraseña"})
		return
	}

	user := &models.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
	}

	if err := h.db.CreateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error creando usuario"})
		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error generando token"})
		return
	}

	c.JSON(http.StatusCreated, models.AuthResponse{Token: token, User: *user})
}

// Login maneja POST /api/auth/login
// Verifica credenciales y retorna un token JWT.
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.db.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error interno"})
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciales inválidas"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciales inválidas"})
		return
	}

	token, err := h.generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error generando token"})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{Token: token, User: *user})
}

// Me maneja GET /api/auth/me — retorna el usuario autenticado
func (h *AuthHandler) Me(c *gin.Context) {
	userID := middleware.GetUserID(c)
	// Podrías buscar el usuario en la DB, pero para simplificar usamos el contexto
	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"username": c.GetString("username"),
	})
}

// generateToken crea un JWT firmado con expiración de 24 horas.
func (h *AuthHandler) generateToken(user *models.User) (string, error) {
	claims := &middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
