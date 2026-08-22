package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"okf-worker/converter"
)

type ValidationStatus string

const (
	StatusValid        ValidationStatus = "valid"
	StatusValidWarning ValidationStatus = "valid_with_warnings"
	StatusInvalid      ValidationStatus = "invalid"
)

// ValidationResult contiene el resultado de la validación del bundle.
type ValidationResult struct {
	Valid    bool
	Status   ValidationStatus
	Errors   []string
	Warnings []string
}

// isSuspiciousText evalúa si un fragmento de PROSA parece estar corrupto
// (columnas de PDF mezcladas, HTML crudo sin escapar, mojibake, texto cortado
// a mitad de palabra). Se salta fragmentos menores a 50 caracteres. Sobre los
// caracteres no-espacio cuenta cuántos son letras (letters), cuántos no lo
// son —dígitos, puntuación, símbolos— (nonLetters) y, de las letras, cuántas
// son vocales (vowels). Se marca como sospechoso si:
//  1. no hay ninguna letra en el texto,
//  2. hay más caracteres no alfabéticos que letras (nonLetters > letters), o
//  3. las vocales son menos del 10% de las letras (vowels/letters < 0.10).
//
// Devuelve también la razón exacta (con los números que la dispararon) para
// poder auditar el detector con evidencia real en vez de adivinar por qué se
// activó. Debe llamarse solo con prosa (ver extractProseText): el contenido
// de tablas, listas y bloques de código tiene alta densidad "natural" de
// guiones, dígitos y símbolos por su propia sintaxis, y evaluarlo con estas
// mismas reglas produce falsos positivos sistemáticos (p. ej. una tabla de
// datos con celdas tipo "palabra-12-0-0" dispara la condición 2 sin que haya
// ningún problema real de conversión).
func isSuspiciousText(text string) (bool, string) {
	if len(text) < 50 {
		return false, ""
	}

	letters := 0
	nonLetters := 0
	vowels := 0

	for _, ch := range text {
		if unicode.IsSpace(ch) {
			continue
		}
		if unicode.IsLetter(ch) {
			letters++
			lower := unicode.ToLower(ch)
			if lower == 'a' || lower == 'e' || lower == 'i' || lower == 'o' || lower == 'u' ||
				lower == 'á' || lower == 'é' || lower == 'í' || lower == 'ó' || lower == 'ú' {
				vowels++
			}
		} else {
			nonLetters++
		}
	}

	if letters == 0 {
		return true, "no contiene ninguna letra"
	}

	if nonLetters > letters {
		return true, fmt.Sprintf("más caracteres no alfabéticos que letras (no alfabéticos: %d, letras: %d)", nonLetters, letters)
	}

	ratio := float64(vowels) / float64(letters)
	if ratio < 0.10 {
		return true, fmt.Sprintf("ratio de vocales = %.2f, por debajo del umbral 0.10 (vocales: %d, letras: %d)", ratio, vowels, letters)
	}

	return false, ""
}

// fenceLineRegex reconoce una línea delimitadora de bloque de código fenced
// (``` o ```<lenguaje>), igual que el segmentador de Markdown.
var fenceLineRegex = regexp.MustCompile("^```\\S*$")

// orderedListItemRegex reconoce un ítem de lista ordenada ("1. ", "2. ", …).
var orderedListItemRegex = regexp.MustCompile(`^\d+\.\s`)

// isTableRowLine reconoce una fila de tabla Markdown generada por este
// pipeline: siempre tiene la forma "| celda | celda | ... |".
func isTableRowLine(trimmed string) bool {
	return len(trimmed) >= 2 && strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

// isListItemLine reconoce un ítem de lista Markdown ("- " o "N. "),
// tolerando la indentación usada para listas anidadas.
func isListItemLine(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	if strings.HasPrefix(trimmed, "- ") {
		return true
	}
	return orderedListItemRegex.MatchString(trimmed)
}

// extractProseText retorna solo las líneas de prosa de un concepto,
// excluyendo bloques de código fenced, filas de tabla Markdown e ítems de
// lista. Esas secciones tienen su propia sintaxis estructural —guiones,
// pipes, números— y no deben pasar por el mismo detector de texto corrupto
// pensado para párrafos de prosa corrida (ver isSuspiciousText).
func extractProseText(content string) string {
	lines := strings.Split(content, "\n")
	var prose []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if fenceLineRegex.MatchString(trimmed) {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}
		if isTableRowLine(trimmed) {
			continue
		}
		if isListItemLine(line) {
			continue
		}
		prose = append(prose, line)
	}

	return strings.Join(prose, "\n")
}

// Validate verifica que el bundle ZIP cumple la estructura mínima OKF:
// - Debe contener index.md
// - Debe contener log.md
// - Debe contener al menos un archivo de concepto (.md que no sea index.md ni log.md)
// - Todos los enlaces en index.md deben resolverse a archivos existentes en el ZIP
//
// units son las unidades a partir de las cuales se generó el bundle; se usan
// para detectar las condiciones que degradan el resultado a "válido con
// advertencias" (ver detectWarnings).
func Validate(zipData []byte, units []converter.Unit) (*ValidationResult, error) {
	result := &ValidationResult{Valid: true}

	// Leer el ZIP en memoria
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("ZIP inválido: %w", err)
	}

	// Indexar todos los archivos presentes
	presentFiles := make(map[string]bool)
	var indexContent string
	conceptCount := 0

	for _, f := range r.File {
		name := f.Name
		
		if presentFiles[name] {
			result.Errors = append(result.Errors, fmt.Sprintf("FALLO: el archivo '%s' aparece duplicado dentro del ZIP", name))
			result.Valid = false
		}
		
		presentFiles[name] = true

		if name == "index.md" {
			rc, err := f.Open()
			if err != nil {
				result.Errors = append(result.Errors, "no se pudo leer index.md")
				result.Valid = false
				continue
			}
			var buf bytes.Buffer
			buf.ReadFrom(rc)
			rc.Close()
			indexContent = buf.String()
		}

		// Contar conceptos (archivos .md que no son index.md ni log.md)
		if strings.HasSuffix(name, ".md") && name != "index.md" && name != "log.md" {
			conceptCount++
		}
	}

	// Verificación 1: index.md obligatorio
	if !presentFiles["index.md"] {
		result.Errors = append(result.Errors, "FALLO: index.md no existe en el bundle")
		result.Valid = false
	}

	// Verificación 2: log.md obligatorio
	if !presentFiles["log.md"] {
		result.Errors = append(result.Errors, "FALLO: log.md no existe en el bundle")
		result.Valid = false
	}

	// Verificación 3: al menos un concepto
	if conceptCount == 0 {
		result.Errors = append(result.Errors, "FALLO: el bundle no contiene ningún concepto (.md)")
		result.Valid = false
	}

	// Verificación 4: resolución de enlaces en index.md
	if indexContent != "" {
		brokenLinks := checkLinks(indexContent, presentFiles)
		for _, link := range brokenLinks {
			result.Errors = append(result.Errors, fmt.Sprintf("FALLO: enlace roto en index.md → '%s'", link))
			result.Valid = false
		}
	}

	// Advertencias (no impiden publicación)
	result.Warnings = append(result.Warnings, detectWarnings(units)...)

	if !result.Valid {
		result.Status = StatusInvalid
	} else if len(result.Warnings) > 0 {
		result.Status = StatusValidWarning
	} else {
		result.Status = StatusValid
	}

	return result, nil
}

// detectWarnings evalúa, sobre las unidades ya generadas, las condiciones que
// degradan el bundle a "válido con advertencias": slugs de respaldo, texto
// sospechoso de estar corrupto (evaluado solo sobre la prosa del concepto,
// ver extractProseText), bloques de código sin cerrar, conceptos sin
// contenido, títulos duplicados y documentos de un único concepto. La usan
// tanto Validate() —para decidir el estado del bundle publicado— como
// bundle.Generate() —para que log.md registre el mismo veredicto en vez de
// asumir siempre éxito. En particular, log.md nunca cierra con "sin
// advertencias" si al menos un concepto quedó con 0 caracteres de contenido,
// porque esa condición siempre agrega una advertencia aquí.
func detectWarnings(units []converter.Unit) []string {
	var warnings []string
	titleCounts := make(map[string]int)
	for _, u := range units {
		titleCounts[strings.TrimSpace(u.Title)]++
	}

	emptyCount := 0
	seenDuplicateTitle := make(map[string]bool)
	for _, u := range units {
		if u.FallbackSlugUsed {
			warnings = append(warnings, fmt.Sprintf("Se usó slug de respaldo en %s", u.Slug))
		}
		if suspicious, reason := isSuspiciousText(extractProseText(u.Content)); suspicious {
			warnings = append(warnings, fmt.Sprintf("Texto sospechoso detectado en %s (razón: %s)", u.Slug, reason))
		}
		if strings.TrimSpace(u.Content) == "" {
			emptyCount++
			warnings = append(warnings, fmt.Sprintf("Concepto sin contenido textual: %s", u.Slug))
		}
		for _, note := range u.Notes {
			if strings.Contains(note, "sin cerrar") {
				warnings = append(warnings, fmt.Sprintf("%s (unidad: %s)", note, u.Slug))
			}
		}
		title := strings.TrimSpace(u.Title)
		if titleCounts[title] > 1 && !seenDuplicateTitle[title] {
			seenDuplicateTitle[title] = true
			warnings = append(warnings, fmt.Sprintf("Título duplicado: '%s' aparece en %d conceptos", title, titleCounts[title]))
		}
	}
	if len(units) == 1 {
		warnings = append(warnings, "el bundle contiene un solo concepto (documento breve)")
	}
	if len(units) > 0 && float64(emptyCount)/float64(len(units)) > 0.20 {
		warnings = append(warnings, fmt.Sprintf(
			"Posible fallo de segmentación: %d de %d conceptos (%.0f%%) quedaron sin contenido",
			emptyCount, len(units), 100*float64(emptyCount)/float64(len(units))))
	}
	return warnings
}

// checkLinks extrae y verifica los enlaces Markdown de un archivo.
// Retorna la lista de enlaces que no se resuelven a archivos existentes.
func checkLinks(content string, files map[string]bool) []string {
	var broken []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		// Buscar patrones de enlace Markdown: [texto](archivo.md)
		for {
			start := strings.Index(line, "](")
			if start == -1 {
				break
			}
			rest := line[start+2:]
			end := strings.Index(rest, ")")
			if end == -1 {
				break
			}
			link := rest[:end]
			// Solo verificar enlaces relativos a archivos .md
			if strings.HasSuffix(link, ".md") && !strings.HasPrefix(link, "http") {
				if !files[link] {
					broken = append(broken, link)
				}
			}
			line = rest[end+1:]
		}
	}
	return broken
}
