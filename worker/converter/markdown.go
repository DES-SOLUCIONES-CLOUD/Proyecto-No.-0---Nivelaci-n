package converter

import (
	"strings"
)

// Unit representa una unidad lógica (concepto) extraída de un documento.
type Unit struct {
	Title   string // Título de la sección
	Content string // Contenido en Markdown
	Slug    string // Nombre de archivo: capitulo-01.md
}

// ParseMarkdown divide un documento Markdown en unidades lógicas por encabezados.
// Cada encabezado H1, H2 o H3 inicia una nueva unidad.
// Si el documento no tiene encabezados, el contenido completo es una sola unidad.
func ParseMarkdown(content string) []Unit {
	lines := strings.Split(content, "\n")
	var units []Unit
	var currentTitle string
	var currentLines []string
	unitIdx := 0

	flush := func() {
		if len(currentLines) > 0 || currentTitle != "" {
			body := strings.TrimSpace(strings.Join(currentLines, "\n"))
			title := currentTitle
			if title == "" {
				title = "Documento"
			}
			slug := makeSlug(unitIdx+1, title)
			units = append(units, Unit{
				Title:   title,
				Content: body,
				Slug:    slug,
			})
			unitIdx++
			currentLines = nil
			currentTitle = ""
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "### ") ||
			strings.HasPrefix(line, "## ") ||
			strings.HasPrefix(line, "# ") {
			flush()
			// Extraer título sin el prefijo #
			currentTitle = strings.TrimSpace(strings.TrimLeft(line, "#"))
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	if len(units) == 0 {
		units = append(units, Unit{
			Title:   "Documento",
			Content: strings.TrimSpace(content),
			Slug:    "documento.md",
		})
	}

	return units
}

// makeSlug genera un nombre de archivo seguro para una unidad.
func makeSlug(idx int, title string) string {
	// Normalizar título a kebab-case
	slug := strings.ToLower(title)
	replacer := strings.NewReplacer(
		" ", "-", "/", "-", "\\", "-",
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
		"ñ", "n", "ü", "u",
		":", "", ",", "", ".", "", "¡", "", "!", "", "¿", "", "?", "",
		"(", "", ")", "", "'", "", "\"", "", "`", "",
	)
	slug = replacer.Replace(slug)
	// Eliminar guiones múltiples
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "concepto"
	}
	return makeFilename(idx, slug)
}

// makeFilename genera el nombre de archivo con índice: concepto-01.md
func makeFilename(idx int, slug string) string {
	// Limitar longitud del slug
	if len(slug) > 30 {
		slug = slug[:30]
	}
	return strings.TrimRight(slug, "-") + ".md"
}
