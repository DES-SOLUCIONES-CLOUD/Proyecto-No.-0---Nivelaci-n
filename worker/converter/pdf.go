package converter

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// sectionHeadingRegex detecta encabezados de sección numerados, p. ej.
// "1. Contexto" o "12) Conclusiones". Solo se evalúa contra la primera línea
// no vacía de cada página: una coincidencia en mitad del cuerpo (listas,
// referencias como "(2)(3)(4)") no cuenta como inicio de sección.
var sectionHeadingRegex = regexp.MustCompile(`^\d{1,2}[.)]\s+\S`)

// minNoisePageLen es el umbral por debajo del cual una página inicial o final
// sin encabezado propio (portada, agenda, contraportada) se fusiona con la
// sección vecina en lugar de publicarse como un concepto aparte.
const minNoisePageLen = 200

// ParsePDF extrae el texto de un PDF y lo divide en unidades lógicas.
//
// La extracción usa el modo por defecto de pdftotext ("orden de lectura"),
// sin la bandera -layout: -layout preserva la posición física línea por línea
// y por eso, en documentos a varias columnas, intercala el texto de una
// columna con el de la otra. El modo por defecto reconstruye el flujo de
// lectura columna por columna.
//
// La segmentación agrupa páginas bajo el encabezado numerado que las inicia
// (p. ej. "1. Introducción"), de modo que una sección de varias diapositivas
// produce un único concepto en vez de uno por página. Si el documento no
// tiene encabezados numerados, se conserva el comportamiento de una unidad
// por página (fusionando las muy cortas), ya que no hay una estructura de
// secciones que detectar.
func ParsePDF(data []byte) ([]Unit, error) {
	text, err := extractPDFText(data)
	if err != nil {
		return nil, err
	}

	pages := splitPages(text)

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
		title, body := extractTitleAndBody(pages[0], 1)
		return []Unit{
			{
				Title:            title,
				Content:          body,
				Slug:             "concepto-01-documento.md",
				FallbackSlugUsed: false,
			},
		}, nil
	}

	return unitsFromPages(pages), nil
}

// extractPDFText invoca pdftotext en modo de orden de lectura (sin -layout).
func extractPDFText(data []byte) (string, error) {
	tmpFile, err := os.CreateTemp("", "upload-*.pdf")
	if err != nil {
		return "", fmt.Errorf("error creando archivo temporal: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		return "", fmt.Errorf("error escribiendo archivo temporal: %w", err)
	}
	tmpFile.Close()

	cmd := exec.Command("pdftotext", tmpFile.Name(), "-")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("error ejecutando pdftotext: %w, stderr: %s", err, stderr.String())
	}

	return out.String(), nil
}

// splitPages separa el texto extraído en páginas no vacías.
func splitPages(text string) []string {
	raw := strings.Split(text, "\x0c")
	var pages []string
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			pages = append(pages, p)
		}
	}
	return pages
}

// unitsFromPages agrupa páginas en unidades lógicas y construye los Unit
// finales. Si ninguna página arranca con un encabezado numerado, se aplica el
// agrupamiento anterior (una unidad por página, fusionando las cortas).
func unitsFromPages(pages []string) []Unit {
	hasNumberedHeadings := false
	for _, p := range pages {
		if sectionHeadingRegex.MatchString(firstLine(p)) {
			hasNumberedHeadings = true
			break
		}
	}

	var grouped []string
	if hasNumberedHeadings {
		grouped = groupBySections(pages)
	} else {
		grouped = mergeSparsePages(pages)
	}

	type titledBody struct {
		title string
		body  string
	}
	extracted := make([]titledBody, len(grouped))
	for i, pageText := range grouped {
		title, body := extractTitleAndBody(pageText, i+1)
		extracted[i] = titledBody{title: title, body: body}
	}

	// Es común que la portada, o una diapositiva de título de capítulo, repita
	// exactamente el mismo encabezado que la sección que le sigue antes de
	// entrar en su contenido real (p. ej. una portada y luego la diapositiva
	// divisoria "1. Introducción", ambas con ese mismo texto). En ese caso la
	// primera unidad es material preliminar, no la sección en sí: se conserva
	// su contenido pero se le retira el título para no atribuírselo por
	// duplicado a costa de la sección real.
	if len(extracted) >= 2 &&
		strings.EqualFold(strings.TrimSpace(extracted[0].title), strings.TrimSpace(extracted[1].title)) {
		extracted[0].title = "Preliminares"
	}

	units := make([]Unit, 0, len(extracted))
	for i, e := range extracted {
		slug, fallback := makeSlug(i+1, e.title)
		units = append(units, Unit{
			Title:            e.title,
			Content:          e.body,
			Slug:             slug,
			FallbackSlugUsed: fallback,
		})
	}
	return units
}

// groupBySections agrupa páginas consecutivas bajo el encabezado numerado que
// las inicia. La portada o agenda anteriores al primer encabezado se fusionan
// con la primera sección real, y una contraportada o dato de contacto tras la
// última sección se fusiona con la sección anterior, en vez de publicarse
// como conceptos aparte.
func groupBySections(pages []string) []string {
	var sections []string
	for _, p := range pages {
		if len(sections) == 0 || sectionHeadingRegex.MatchString(firstLine(p)) {
			sections = append(sections, p)
		} else {
			sections[len(sections)-1] += "\n\n" + p
		}
	}

	if len(sections) > 1 && len(sections[0]) < minNoisePageLen &&
		!sectionHeadingRegex.MatchString(firstLine(sections[0])) {
		front := sections[0]
		sections = sections[1:]
		sections[0] = front + "\n\n" + sections[0]
	}

	if len(sections) > 1 && len(sections[len(sections)-1]) < minNoisePageLen &&
		!sectionHeadingRegex.MatchString(firstLine(sections[len(sections)-1])) {
		last := sections[len(sections)-1]
		sections = sections[:len(sections)-1]
		sections[len(sections)-1] += "\n\n" + last
	}

	return sections
}

// mergeSparsePages agrupa páginas cortas con la siguiente para evitar
// unidades vacías o de muy poco contenido. Una página corta que quede al
// final, sin una siguiente con la que fusionarse, se une a la unidad anterior
// en lugar de publicarse sola.
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
		if len(buf) < minPageLen && len(result) > 0 {
			result[len(result)-1] += "\n\n" + buf
		} else {
			result = append(result, buf)
		}
	}
	return result
}

// firstLine retorna la primera línea no vacía de un texto.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// extractTitleAndBody obtiene el título de una unidad y el cuerpo sin la
// línea de título, para no duplicarla luego en el concepto generado.
//
// Si el texto contiene un encabezado numerado (p. ej. "1. Introducción"), ese
// es el título sin importar en qué línea aparezca: esto permite que una
// portada fusionada por delante conserve su lugar en el cuerpo sin robarle el
// título a la sección. Si no hay encabezado numerado, se usa la primera línea
// no vacía, como en el comportamiento original.
func extractTitleAndBody(text string, pageNum int) (string, string) {
	lines := strings.Split(text, "\n")

	headingIdx := -1
	for i, line := range lines {
		if sectionHeadingRegex.MatchString(strings.TrimSpace(line)) {
			headingIdx = i
			break
		}
	}

	if headingIdx == -1 {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			return truncateTitle(trimmed), removeLine(lines, i)
		}
		return fmt.Sprintf("Página %d", pageNum), text
	}

	title := truncateTitle(strings.TrimSpace(lines[headingIdx]))
	return title, removeLine(lines, headingIdx)
}

// truncateTitle recorta un título demasiado largo para que siga siendo
// razonable como encabezado y como base de un slug de archivo.
func truncateTitle(title string) string {
	if len(title) <= 80 {
		return title
	}
	return title[:60] + "…"
}

// removeLine retorna el texto sin la línea en el índice dado, para evitar que
// la línea de título quede duplicada dentro del cuerpo del concepto.
func removeLine(lines []string, idx int) string {
	rest := make([]string, 0, len(lines)-1)
	rest = append(rest, lines[:idx]...)
	rest = append(rest, lines[idx+1:]...)
	return strings.TrimSpace(strings.Join(rest, "\n"))
}
