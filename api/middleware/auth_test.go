package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testSecret = "test-secret-para-pruebas"

// makeToken firma un JWT con los claims y el secreto dados, para poder
// probar el middleware sin pasar por el handler de login.
func makeToken(t *testing.T, secret string, userID, username string, expiresAt time.Time) string {
	t.Helper()
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("no se pudo firmar el token de prueba: %v", err)
	}
	return token
}

// runAuth ejecuta el middleware Auth sobre una petición con el header
// Authorization dado y retorna el código de estado, el cuerpo, y el userID
// que el middleware haya dejado en el contexto (si llegó a dejar pasar la petición).
func runAuth(t *testing.T, secret, authHeader string) (status int, body string, userID string, username string) {
	t.Helper()

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)

	var gotUserID, gotUsername string
	r.GET("/protegido", Auth(secret), func(c *gin.Context) {
		gotUserID = GetUserID(c)
		gotUsername = c.GetString("username")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	c.Request = req
	r.ServeHTTP(w, req)

	return w.Code, w.Body.String(), gotUserID, gotUsername
}

func TestAuth_RejectsMissingHeader(t *testing.T) {
	status, body, _, _ := runAuth(t, testSecret, "")
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, se esperaba 401", status)
	}
	if !strings.Contains(body, "token requerido") {
		t.Errorf("body = %q, se esperaba mensaje 'token requerido'", body)
	}
}

func TestAuth_RejectsMalformedHeader(t *testing.T) {
	tests := []string{
		"TokenSinBearer",
		"Bearer",             // sin el token
		"Basic dXNlcjpwd2==", // esquema distinto
	}
	for _, h := range tests {
		t.Run(h, func(t *testing.T) {
			status, body, _, _ := runAuth(t, testSecret, h)
			if status != http.StatusUnauthorized {
				t.Errorf("header %q: status = %d, se esperaba 401", h, status)
			}
			if !strings.Contains(body, "token requerido") && !strings.Contains(body, "formato de token inválido") {
				t.Errorf("header %q: body = %q, se esperaba error de formato/token", h, body)
			}
		})
	}
}

func TestAuth_RejectsGarbageToken(t *testing.T) {
	status, body, _, _ := runAuth(t, testSecret, "Bearer esto-no-es-un-jwt-valido")
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, se esperaba 401", status)
	}
	if !strings.Contains(body, "token inválido o expirado") {
		t.Errorf("body = %q, se esperaba 'token inválido o expirado'", body)
	}
}

func TestAuth_RejectsExpiredToken(t *testing.T) {
	token := makeToken(t, testSecret, "user-1", "alice", time.Now().Add(-1*time.Hour))
	status, body, _, _ := runAuth(t, testSecret, "Bearer "+token)
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, se esperaba 401 para un token expirado", status)
	}
	if !strings.Contains(body, "token inválido o expirado") {
		t.Errorf("body = %q, se esperaba 'token inválido o expirado'", body)
	}
}

// Un token firmado con un secreto DISTINTO al configurado en el servidor
// nunca debe aceptarse — es la base de que un usuario no pueda forjar
// credenciales de otro.
func TestAuth_RejectsTokenSignedWithWrongSecret(t *testing.T) {
	token := makeToken(t, "otro-secreto-cualquiera", "user-1", "alice", time.Now().Add(1*time.Hour))
	status, _, _, _ := runAuth(t, testSecret, "Bearer "+token)
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, se esperaba 401 para un token firmado con otro secreto", status)
	}
}

func TestAuth_AcceptsValidTokenAndPropagatesClaims(t *testing.T) {
	token := makeToken(t, testSecret, "user-42", "bob", time.Now().Add(1*time.Hour))
	status, _, userID, username := runAuth(t, testSecret, "Bearer "+token)
	if status != http.StatusOK {
		t.Fatalf("status = %d, se esperaba 200 para un token válido", status)
	}
	if userID != "user-42" {
		t.Errorf("userID en contexto = %q, se esperaba 'user-42'", userID)
	}
	if username != "bob" {
		t.Errorf("username en contexto = %q, se esperaba 'bob'", username)
	}
}

// El esquema Bearer no debe ser sensible a mayúsculas/minúsculas (algunos
// clientes HTTP normalizan el header de forma distinta).
func TestAuth_BearerSchemeIsCaseInsensitive(t *testing.T) {
	token := makeToken(t, testSecret, "user-1", "alice", time.Now().Add(1*time.Hour))
	status, _, _, _ := runAuth(t, testSecret, "bearer "+token)
	if status != http.StatusOK {
		t.Errorf("status = %d, se esperaba 200 con esquema 'bearer' en minúsculas", status)
	}
}
