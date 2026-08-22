package bundle

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"okf-worker/converter"
)

func TestGenerate_NoDuplicateFiles(t *testing.T) {
	units := []converter.Unit{
		{
			Title:   "Concepto 1",
			Content: "Contenido 1",
			Slug:    "concepto-01.md",
		},
		{
			Title:   "Concepto 2",
			Content: "Contenido 2",
			Slug:    "concepto-02.md",
		},
	}

	result, err := Generate("job-123", "test.txt", "txt", units)
	if err != nil {
		t.Fatalf("Generate falló: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(result.ZipData), int64(len(result.ZipData)))
	if err != nil {
		t.Fatalf("No se pudo leer el ZIP: %v", err)
	}

	fileCounts := make(map[string]int)
	for _, f := range r.File {
		fileCounts[f.Name]++
	}

	// Verificar que cada archivo aparece exactamente una vez
	for name, count := range fileCounts {
		if count > 1 {
			t.Errorf("El archivo %s aparece %d veces en el ZIP generado", name, count)
		}
	}

	// Verificar la presencia de archivos obligatorios
	if fileCounts["index.md"] != 1 {
		t.Errorf("Se esperaba exactamente 1 index.md, pero hay %d", fileCounts["index.md"])
	}
	if fileCounts["log.md"] != 1 {
		t.Errorf("Se esperaba exactamente 1 log.md, pero hay %d", fileCounts["log.md"])
	}
	if fileCounts["concepto-01.md"] != 1 {
		t.Errorf("Se esperaba exactamente 1 concepto-01.md, pero hay %d", fileCounts["concepto-01.md"])
	}
}

// El log.md no debe afirmar "sin advertencias" cuando en realidad detectó
// condiciones de advertencia (aquí, títulos duplicados) — el veredicto que
// registra debe coincidir con el que luego calculará Validate().
func TestGenerate_LogReflectsRealWarnings(t *testing.T) {
	units := []converter.Unit{
		{Title: "Introducción", Content: "Contenido 1", Slug: "concepto-01-introduccion.md"},
		{Title: "Introducción", Content: "Contenido 2", Slug: "concepto-02-introduccion.md"},
	}

	result, err := Generate("job-123", "test.txt", "txt", units)
	if err != nil {
		t.Fatalf("Generate falló: %v", err)
	}

	logContent := readZipFile(t, result.ZipData, "log.md")

	if strings.Contains(logContent, "sin advertencias") {
		t.Errorf("log.md afirma 'sin advertencias' pero hay títulos duplicados:\n%s", logContent)
	}
	if !strings.Contains(logContent, "Título duplicado") {
		t.Errorf("log.md no registró la advertencia de título duplicado:\n%s", logContent)
	}

	warnings := detectWarnings(units)
	validation, err := Validate(result.ZipData, units)
	if err != nil {
		t.Fatalf("Validate falló: %v", err)
	}
	if len(validation.Warnings) != len(warnings) {
		t.Errorf("Validate() encontró %d advertencias pero detectWarnings() encontró %d — deberían coincidir", len(validation.Warnings), len(warnings))
	}
	if validation.Status != StatusValidWarning {
		t.Errorf("Status = %q, se esperaba %q dado que hay advertencias", validation.Status, StatusValidWarning)
	}
}

func readZipFile(t *testing.T, zipData []byte, name string) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("no se pudo leer el ZIP: %v", err)
	}
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("no se pudo abrir %s: %v", name, err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("no se pudo leer %s: %v", name, err)
			}
			return string(data)
		}
	}
	t.Fatalf("%s no está en el ZIP", name)
	return ""
}
