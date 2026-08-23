package config

import "testing"

func TestGetEnv_UsesEnvironmentValueWhenSet(t *testing.T) {
	t.Setenv("OKF_TEST_VAR", "valor-real")
	if got := getEnv("OKF_TEST_VAR", "fallback"); got != "valor-real" {
		t.Errorf("getEnv() = %q, se esperaba 'valor-real'", got)
	}
}

func TestGetEnv_FallsBackWhenUnset(t *testing.T) {
	if got := getEnv("OKF_TEST_VAR_QUE_NO_EXISTE", "fallback"); got != "fallback" {
		t.Errorf("getEnv() = %q, se esperaba el valor por defecto 'fallback'", got)
	}
}

func TestGetEnv_FallsBackWhenEmpty(t *testing.T) {
	t.Setenv("OKF_TEST_VAR_VACIA", "")
	if got := getEnv("OKF_TEST_VAR_VACIA", "fallback"); got != "fallback" {
		t.Errorf("getEnv() = %q, una variable vacía debería usar el valor por defecto", got)
	}
}

// Load() no debe fallar sin ninguna variable de entorno configurada: todos
// los valores por defecto deben ser suficientes para levantar el servicio en
// desarrollo local (documentado en el propio comentario de Load()).
func TestLoad_ProducesUsableDefaults(t *testing.T) {
	cfg := Load()
	if cfg.Port == "" {
		t.Error("Port no debería quedar vacío por defecto")
	}
	if cfg.JWTSecret == "" {
		t.Error("JWTSecret no debería quedar vacío por defecto")
	}
	if cfg.PostgresURL == "" || cfg.RedisURL == "" || cfg.MinioEndpoint == "" {
		t.Error("las URLs de dependencias no deberían quedar vacías por defecto")
	}
}
