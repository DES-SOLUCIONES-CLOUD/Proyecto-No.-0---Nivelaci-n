package converter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ParsePDF extrae el texto de un PDF y lo divide en unidades lógicas por página.
// Si el PDF tiene una sola página o el texto no supera el umbral mínimo,
// todo el contenido se retorna como una única unidad.
// Retorna error si el PDF es ilegible (p. ej. PDF escaneado sin capa de texto).
func ParsePDF(data []byte) ([]Unit, error) {
	tmpFile, err := os.CreateTemp("", "upload-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("error creando archivo temporal: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		return nil, fmt.Errorf("error escribiendo archivo temporal: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command("pdftotext", "-layout", tmpFile.Name(), "-")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("error ejecutando pdftotext: %w, stderr: %s", err, stderr.String())
	}

	text := out.String()
	rawPages := strings.Split(text, "\x0c")

	var pages []string
	for _, p := range rawPages {
		p = strings.TrimSpace(p)
		if p != "" {
			pages = append(pages, p)
		}
	}

	if len(pages) == 0 {
		return []Unit{
			{
				Title:            "Documento PDF",
				Content:          "*Este PDF no contiene texto seleccionable (posiblemente es una imagen escaneada). No se pudo extraer contenido.*",
				Slug:             "concepto-01-documento.md",
				FallbackSlugUsed: false,
			},
		}, nil
	}

	if len(pages) == 1 {
		return []Unit{
			{
				Title:            "Documento",
				Content:          pages[0],
				Slug:             "concepto-01-documento.md",
				FallbackSlugUsed: false,
			},
		}, nil
	}

	merged := mergeSparsePages(pages)

	units := make([]Unit, 0, len(merged))
	for i, pageText := range merged {
		title := extractPageTitle(pageText, i+1)
		slug, fallback := makeSlug(i+1, title)
		units = append(units, Unit{
			Title:            title,
			Content:          pageText,
			Slug:             slug,
			FallbackSlugUsed: fallback,
		})
	}

	return units, nil
}

// mergeSparsePages agrupa páginas cortas con la siguiente para evitar
// unidades vacías o de muy poco contenido.
func mergeSparsePages(pages []string) []string {
	const minPageLen = 100
	var result []string
	buf := ""

	for _, p := range pages {
		if buf == "" {
			buf = p
		} else if len(buf) < minPageLen {
			buf = buf + "\n\n" + p
		} else {
			result = append(result, buf)
			buf = p
		}
	}
	if buf != "" {
		result = append(result, buf)
	}
	return result
}

// extractPageTitle intenta obtener un título de la primera línea no vacía del texto.
// Si la primera línea es demasiado larga o vacía, usa "Página N".
func extractPageTitle(text string, pageNum int) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Usar la primera línea como título si es razonablemente corta
		if len(line) <= 80 {
			return line
		}
		// Si es muy larga, truncar
		return line[:60] + "…"
	}
	return fmt.Sprintf("Página %d", pageNum)
}
