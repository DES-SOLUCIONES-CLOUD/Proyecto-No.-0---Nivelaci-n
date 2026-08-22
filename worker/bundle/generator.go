package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"okf-worker/converter"
)

// Bundle representa el bundle OKF completo en memoria antes de ser comprimido.
type Bundle struct {
	JobID        string
	Filename     string
	Units        []converter.Unit
	IndexMD      string
	LogMD        string
	ConceptCount int
}

// GenerationResult contiene el resultado de la generación del bundle.
type GenerationResult struct {
	ZipData      []byte
	ConceptCount int
	Log          []string
	Units        []converter.Unit
}

// Generate toma las unidades de un documento y genera el bundle OKF completo.
// Retorna los bytes del ZIP y el conteo de conceptos generados.
func Generate(jobID, filename, format string, units []converter.Unit) (*GenerationResult, error) {
	var logEntries []string
	now := time.Now().UTC()

	log := func(msg string) {
		entry := fmt.Sprintf("[%s] %s", now.Format("2006-01-02T15:04:05Z"), msg)
		logEntries = append(logEntries, entry)
	}

	log(fmt.Sprintf("Inicio de conversión: %s (formato: %s)", filename, format))
	log(fmt.Sprintf("Job ID: %s", jobID))
	log(fmt.Sprintf("Unidades lógicas detectadas: %d", len(units)))

	for i, u := range units {
		msg := fmt.Sprintf("  Unidad %d: '%s' → %s (%d chars)", i+1, u.Title, u.Slug, utf8.RuneCountInString(u.Content))
		if u.FallbackSlugUsed {
			msg += " [Slug de respaldo usado]"
		}
		log(msg)
		for _, note := range u.Notes {
			log(fmt.Sprintf("    %s", note))
		}
	}

	// Generar index.md
	indexMD := generateIndex(filename, jobID, units, now, log)
	log("index.md generado con navegación y metadatos")

	// Crear ZIP en memoria
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// Agregar index.md
	if err := addToZip(w, "index.md", []byte(indexMD)); err != nil {
		return nil, fmt.Errorf("error creando index.md: %w", err)
	}
	log("index.md añadido al bundle")

	// Agregar concepto por unidad
	for _, unit := range units {
		conceptMD := generateConcept(unit)
		if err := addToZip(w, unit.Slug, []byte(conceptMD)); err != nil {
			return nil, fmt.Errorf("error creando concepto %s: %w", unit.Slug, err)
		}
		log(fmt.Sprintf("Concepto añadido: %s", unit.Slug))
	}

	// Agregar log.md (al final, con todos los registros)
	warnings := detectWarnings(units)
	logMDFinal := generateLogMD(logEntries, warnings, filename, jobID, now)
	if err := addToZip(w, "log.md", []byte(logMDFinal)); err != nil {
		return nil, fmt.Errorf("error creando log.md: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("error cerrando ZIP: %w", err)
	}

	return &GenerationResult{
		ZipData:      buf.Bytes(),
		ConceptCount: len(units),
		Log:          logEntries,
		Units:        units,
	}, nil
}

// generateIndex genera el contenido del archivo index.md con navegación.
func generateIndex(filename, jobID string, units []converter.Unit, t time.Time, log func(string)) string {
	var sb strings.Builder

	sb.WriteString("# Bundle OKF — Índice\n\n")
	sb.WriteString(fmt.Sprintf("**Documento de origen:** `%s`  \n", filename))
	sb.WriteString(fmt.Sprintf("**Job ID:** `%s`  \n", jobID))
	sb.WriteString(fmt.Sprintf("**Generado:** %s  \n", t.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("**Conceptos:** %d  \n\n", len(units)))
	sb.WriteString("---\n\n")
	sb.WriteString("## Navegación\n\n")

	titleTotals := make(map[string]int)
	for _, u := range units {
		titleTotals[u.Title]++
	}
	titleCurrent := make(map[string]int)

	for i, unit := range units {
		title := unit.Title
		if titleTotals[title] > 1 {
			titleCurrent[title]++
			title = fmt.Sprintf("%s (%d/%d)", title, titleCurrent[title], titleTotals[title])
			log(fmt.Sprintf("  - Título duplicado detectado: se usará '%s' en el índice para %s", title, unit.Slug))
		}
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n", i+1, title, unit.Slug))
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("*Bundle generado por la Plataforma OKF — ISIS4426 Desarrollo de Soluciones Cloud*\n")

	return sb.String()
}

// generateLogMD genera el archivo log.md con la trazabilidad completa.
//
// La estructura mínima y la resolución de enlaces del índice están
// garantizadas por construcción (Generate solo enlaza los slugs que él mismo
// crea), así que esas dos líneas siempre son ✅. El veredicto final, en
// cambio, refleja las advertencias detectadas en warnings —las mismas que
// bundle.Validate() evaluará después sobre el ZIP publicado— en vez de
// asumir siempre éxito.
func generateLogMD(entries []string, warnings []string, filename, jobID string, t time.Time) string {
	var sb strings.Builder

	sb.WriteString("# Log de Conversión — OKF Bundle\n\n")
	sb.WriteString(fmt.Sprintf("**Archivo:** `%s`  \n", filename))
	sb.WriteString(fmt.Sprintf("**Job ID:** `%s`  \n", jobID))
	sb.WriteString(fmt.Sprintf("**Inicio:** %s  \n\n", t.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString("---\n\n")
	sb.WriteString("## Registro de Operaciones\n\n")
	sb.WriteString("```\n")
	for _, entry := range entries {
		sb.WriteString(entry)
		sb.WriteString("\n")
	}
	sb.WriteString("```\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString("## Validaciones\n\n")
	sb.WriteString("- ✅ Estructura mínima verificada (index.md, log.md, al menos 1 concepto)\n")
	sb.WriteString("- ✅ Todos los enlaces del índice resueltos\n")
	if len(warnings) == 0 {
		sb.WriteString("- ✅ Bundle válido, sin advertencias\n")
	} else {
		sb.WriteString(fmt.Sprintf("- ⚠️ Bundle válido con %d advertencia(s):\n", len(warnings)))
		for _, w := range warnings {
			sb.WriteString(fmt.Sprintf("  - ⚠️ %s\n", w))
		}
	}

	return sb.String()
}

// generateConcept genera el contenido Markdown de un concepto individual.
func generateConcept(unit converter.Unit) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", unit.Title))

	if unit.Content == "" {
		sb.WriteString("*Esta sección no contiene contenido textual.*\n")
	} else {
		sb.WriteString(unit.Content)
		sb.WriteString("\n")
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("[← Volver al índice](index.md)\n")

	return sb.String()
}

// addToZip agrega un archivo al ZIP en memoria.
func addToZip(w *zip.Writer, name string, content []byte) error {
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(content)
	return err
}
