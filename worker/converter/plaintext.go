package converter

import (
	"strings"
	"unicode"
)

// ParsePlainText divide un documento de texto plano en unidades lógicas.
// La segmentación usa estas heurísticas (en orden de prioridad):
//  1. Líneas completamente en MAYÚSCULAS con más de 3 caracteres → nuevo título
//  2. Líneas que terminan con ":" y son cortas (<= 60 chars) → nuevo título
//  3. Bloques separados por 2+ líneas en blanco consecutivas
func ParsePlainText(content string) []Unit {
	lines := strings.Split(content, "\n")

	var units []Unit
	var currentTitle string
	var currentLines []string
	idx := 0

	flush := func() {
		body := strings.TrimSpace(strings.Join(currentLines, "\n"))
		title := currentTitle
		if title == "" {
			title = "Documento"
		}
		if body != "" || title != "Documento" {
			units = append(units, Unit{
				Title:   title,
				Content: body,
				Slug:    makeSlug(idx+1, title),
			})
			idx++
		}
		currentLines = nil
		currentTitle = ""
	}

	consecutiveBlanks := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			consecutiveBlanks++
			currentLines = append(currentLines, line)
			continue
		}

		// Heurística 1: línea en MAYÚSCULAS
		if isAllCaps(trimmed) && len(trimmed) > 3 {
			flush()
			currentTitle = toTitleCase(trimmed)
			consecutiveBlanks = 0
			continue
		}

		// Heurística 2: línea corta terminada en ":"
		if strings.HasSuffix(trimmed, ":") && len(trimmed) <= 60 {
			flush()
			currentTitle = strings.TrimSuffix(trimmed, ":")
			consecutiveBlanks = 0
			continue
		}

		// Heurística 3: separación por doble línea en blanco
		if consecutiveBlanks >= 2 && len(currentLines) > 0 {
			// Solo crear nueva unidad si hay suficiente contenido acumulado
			body := strings.TrimSpace(strings.Join(currentLines, "\n"))
			if len(body) > 50 {
				flush()
			}
		}

		consecutiveBlanks = 0
		currentLines = append(currentLines, line)
	}
	flush()

	if len(units) == 0 {
		units = []Unit{{
			Title:   "Documento",
			Content: strings.TrimSpace(content),
			Slug:    "documento.md",
		}}
	}

	return units
}

// isAllCaps verifica si una línea está completamente en mayúsculas (ignorando espacios y números).
func isAllCaps(s string) bool {
	hasAlpha := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasAlpha = true
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return hasAlpha
}

// toTitleCase convierte una cadena en Title Case.
func toTitleCase(s string) string {
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(string(w[0])) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
