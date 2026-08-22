package converter

import (
	"fmt"
	"regexp"
	"strings"
)

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9-]+`)

// fenceRegex reconoce una línea delimitadora de bloque de código fenced:
// ``` sola, o ```<lenguaje> (python, bash, js, yaml, c++, etc.), sin importar
// cuál sea el lenguaje o si no se especifica ninguno.
var fenceRegex = regexp.MustCompile("^```\\S*$")

// Unit representa una unidad lógica (concepto) extraída de un documento.
type Unit struct {
	Title            string   // Título de la sección
	Content          string   // Contenido en Markdown
	Slug             string   // Nombre de archivo: capitulo-01.md
	FallbackSlugUsed bool     // Indica si el slug se generó por respaldo (corrupción)
	Notes            []string // Notas de conversión (p. ej. tablas/listas detectadas) para log.md
}

// ParseMarkdown divide un documento Markdown en unidades lógicas por encabezados.
// Cada encabezado H1, H2 o H3 inicia una nueva unidad.
// Si el documento no tiene encabezados, el contenido completo es una sola unidad.
//
// Mientras el recorrido está dentro de un bloque de código fenced (``` … ```),
// ninguna línea se evalúa como posible encabezado, sin importar con qué
// carácter empiece: así un comentario "# ..." de Python/Bash dentro del
// bloque no se confunde con un límite de sección, y el bloque nunca queda
// partido entre dos conceptos distintos.
func ParseMarkdown(content string) []Unit {
	lines := strings.Split(content, "\n")
	var units []Unit
	var currentTitle string
	var currentLines []string
	var currentNotes []string
	unitIdx := 0
	inCodeBlock := false
	codeBlockStartLine := 0

	flush := func() {
		if len(currentLines) > 0 || currentTitle != "" {
			body := strings.TrimSpace(strings.Join(currentLines, "\n"))
			title := currentTitle
			if title == "" {
				title = "Documento"
			}
			slug, fallback := makeSlug(unitIdx+1, title)
			units = append(units, Unit{
				Title:            title,
				Content:          body,
				Slug:             slug,
				FallbackSlugUsed: fallback,
				Notes:            currentNotes,
			})
			unitIdx++
			currentLines = nil
			currentNotes = nil
			currentTitle = ""
		}
	}

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)

		if fenceRegex.MatchString(trimmed) {
			inCodeBlock = !inCodeBlock
			if inCodeBlock {
				codeBlockStartLine = lineNum + 1 // 1-indexado para el mensaje de log
			}
			currentLines = append(currentLines, line)
			continue
		}

		if inCodeBlock {
			currentLines = append(currentLines, line)
			continue
		}

		// Comprobar si es un encabezado markdown: uno o más '#' seguidos de un espacio
		isHeader := false
		if strings.HasPrefix(trimmed, "#") {
			// Contar hashes
			hashCount := 0
			for _, ch := range trimmed {
				if ch == '#' {
					hashCount++
				} else {
					break
				}
			}
			// Verificar que después de los hashes haya un espacio
			if hashCount > 0 && len(trimmed) > hashCount && trimmed[hashCount] == ' ' {
				isHeader = true
			}
		}

		if isHeader {
			flush()
			// Extraer título sin el prefijo #
			currentTitle = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		} else {
			currentLines = append(currentLines, line)
		}
	}

	if inCodeBlock {
		currentNotes = append(currentNotes, fmt.Sprintf("Bloque de código sin cerrar detectado cerca de la línea %d", codeBlockStartLine))
	}
	flush()

	if len(units) == 0 {
		units = append(units, Unit{
			Title:            "Documento",
			Content:          strings.TrimSpace(content),
			Slug:             "concepto-01-documento.md",
			FallbackSlugUsed: false,
		})
	}

	return units
}

// makeSlug genera un nombre de archivo seguro para una unidad.
// Retorna el slug sanitizado y un booleano indicando si usó slug de respaldo.
func makeSlug(idx int, title string) (string, bool) {
	slug := strings.ToLower(title)
	replacer := strings.NewReplacer(
		" ", "-", "/", "-", "\\", "-",
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u",
		"ñ", "n", "ü", "u",
	)
	slug = replacer.Replace(slug)
	
	// Eliminar cualquier otro carácter que no sea a-z, 0-9 o guión
	slug = nonAlphanumericRegex.ReplaceAllString(slug, "-")
	
	// Eliminar guiones múltiples
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	
	// Contar letras (caracteres alfabéticos a-z)
	alphaCount := 0
	for _, ch := range slug {
		if ch >= 'a' && ch <= 'z' {
			alphaCount++
		}
	}
	
	fallbackUsed := false
	if alphaCount < 3 {
		slug = "concepto"
		fallbackUsed = true
	}
	
	return makeFilename(idx, slug), fallbackUsed
}

// makeFilename genera el nombre de archivo con índice: concepto-01.md
func makeFilename(idx int, slug string) string {
	if len(slug) > 30 {
		slug = slug[:30]
	}
	return fmt.Sprintf("concepto-%02d-%s.md", idx, strings.TrimRight(slug, "-"))
}
