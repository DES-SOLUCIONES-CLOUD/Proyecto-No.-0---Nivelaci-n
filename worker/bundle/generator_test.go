package bundle

import (
	"archive/zip"
	"bytes"
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
