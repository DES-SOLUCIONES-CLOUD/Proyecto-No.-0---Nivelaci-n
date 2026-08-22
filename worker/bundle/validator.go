package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"
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

// isSuspiciousText evalúa si un texto parece estar corrupto (mojibake).
func isSuspiciousText(text string) bool {
	if len(text) < 50 {
		return false
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
		return true
	}
	
	// Si hay demasiados caracteres no alfabéticos (ej. !@#$) respecto a letras
	if nonLetters > letters {
		return true
	}
	
	// Si hay muy pocas vocales (menos del 10% de las letras en textos largos)
	if float64(vowels)/float64(letters) < 0.10 {
		return true
	}
	
	return false
}

// Validate verifica que el bundle ZIP cumple la estructura mínima OKF:
// - Debe contener index.md
// - Debe contener log.md
// - Debe contener al menos un archivo de concepto (.md que no sea index.md ni log.md)
// - Todos los enlaces en index.md deben resolverse a archivos existentes en el ZIP
func Validate(zipData []byte) (*ValidationResult, error) {
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
			
			// Validar si usa slug de respaldo
			fallbackRegex := regexp.MustCompile(`^concepto-\d{2}\.md$`)
			if fallbackRegex.MatchString(name) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Se usó slug de respaldo en %s", name))
			}
			
			// Validar texto sospechoso
			rc, err := f.Open()
			if err == nil {
				var buf bytes.Buffer
				buf.ReadFrom(rc)
				rc.Close()
				if isSuspiciousText(buf.String()) {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Texto sospechoso detectado en %s", name))
				}
			}
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
	if conceptCount == 1 {
		result.Warnings = append(result.Warnings, "el bundle contiene un solo concepto (documento breve)")
	}

	if !result.Valid {
		result.Status = StatusInvalid
	} else if len(result.Warnings) > 0 {
		result.Status = StatusValidWarning
	} else {
		result.Status = StatusValid
	}

	return result, nil
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
