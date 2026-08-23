package converter

import (
	"fmt"
	"regexp"
	"strings"
)

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9-]+`)

// Unit representa una unidad lógica (concepto) extraída de un documento.
type Unit struct {
	Title            string   // Título de la sección
	Content          string   // Contenido en Markdown
	Slug             string   // Nombre de archivo: capitulo-01.md
	FallbackSlugUsed bool     // Indica si el slug se generó por respaldo (corrupción)
	Notes            []string // Notas de conversión (p. ej. tablas/listas detectadas) para log.md
}

// ParseMarkdown divide un documento Markdown en unidades lógicas por
// encabezados ATX ("#", "##", "###", …). Cada encabezado inicia una nueva
// unidad. Si el documento no tiene encabezados, el contenido completo es una
// sola unidad.
//
// Mientras el recorrido está dentro de un bloque de código —fenced con
// backticks (```), fenced con tildes (~~~), o indentado con 4 espacios/un
// tab— ninguna línea se evalúa como encabezado, sin importar con qué
// carácter empiece: así un comentario "# ..." dentro del bloque no se
// confunde con un límite de sección, y el bloque nunca queda partido entre
// dos conceptos. Un bloque abierto con ``` solo se cierra con ```, y uno
// abierto con ~~~ solo con ~~~ (ver matchFenceClose): el delimitador
// contrario que aparezca adentro es contenido literal del bloque.
//
// Si un bloque fenced nunca encuentra su cierre real, se aplica una
// recuperación por límite estructural: al toparse, dentro del bloque
// presuntamente abierto, con un encabezado de nivel igual o superior al de
// la sección donde abrió el bloque, se asume que el bloque terminó
// implícitamente justo antes de ese encabezado. Se cierra de forma
// sintética en la salida y la segmentación continúa con normalidad desde
// ahí, para que un bloque sin cerrar nunca absorba el resto del documento.
// La advertencia queda registrada en el log tanto si se recupera a mitad de
// documento como si el bloque nunca cierra hasta el final del archivo.
func ParseMarkdown(content string) []Unit {
	lines := strings.Split(content, "\n")
	var units []Unit
	var currentTitle string
	var currentLevel int
	var currentLines []string
	var currentNotes []string
	unitIdx := 0

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

	// Estado del bloque de código indentado actualmente abierto (si lo hay).
	inIndented := false
	var indentedLines []string

	closeIndentedBlock := func() {
		for len(indentedLines) > 0 && indentedLines[len(indentedLines)-1] == "" {
			indentedLines = indentedLines[:len(indentedLines)-1]
		}
		if len(indentedLines) > 0 {
			currentLines = append(currentLines, "```")
			currentLines = append(currentLines, indentedLines...)
			currentLines = append(currentLines, "```", "")
			currentNotes = append(currentNotes, fmt.Sprintf("Bloque de código indentado convertido a Markdown (%d líneas)", len(indentedLines)))
		}
		indentedLines = nil
		inIndented = false
	}

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if inIndented {
			if trimmed == "" {
				indentedLines = append(indentedLines, "")
				i++
				continue
			}
			if isIndentedCodeLine(line) {
				indentedLines = append(indentedLines, dedentCodeLine(line))
				i++
				continue
			}
			closeIndentedBlock()
			continue // reprocesar esta línea: ya no es parte del bloque
		}

		if trimmed == "" {
			currentLines = append(currentLines, line)
			i++
			continue
		}

		if ch, fenceLen := matchFenceOpen(trimmed); ch != 0 {
			openLine := i + 1
			body, endIdx, closed, recoveredAtLine := scanFencedBlock(lines, i+1, ch, fenceLen, currentLevel)

			currentLines = append(currentLines, line)
			currentLines = append(currentLines, body...)

			if closed {
				currentLines = append(currentLines, lines[endIdx])
				i = endIdx + 1
				continue
			}

			// Ningún cierre real existe en el resto del documento: se
			// sintetiza el cierre en la salida y se retoma la segmentación
			// normal desde el límite de recuperación (o desde el final del
			// archivo si tampoco hubo un encabezado que sirviera de límite).
			currentLines = append(currentLines, strings.Repeat(string(ch), fenceLen))
			if recoveredAtLine > 0 {
				currentNotes = append(currentNotes, fmt.Sprintf(
					"Bloque de código sin cerrar detectado cerca de la línea %d; se asumió cierre implícito antes del encabezado de la línea %d",
					openLine, recoveredAtLine))
			} else {
				currentNotes = append(currentNotes, fmt.Sprintf("Bloque de código sin cerrar detectado cerca de la línea %d", openLine))
			}
			i = endIdx
			continue
		}

		if isHeader, title := markdownHeaderTitle(trimmed); isHeader {
			flush()
			currentTitle = title
			currentLevel = atxLevel(trimmed)
			i++
			continue
		}

		// Bloque de código indentado: arranca solo si la línea anterior fue
		// una línea en blanco.
		if isIndentedCodeLine(line) && (i == 0 || strings.TrimSpace(lines[i-1]) == "") {
			inIndented = true
			indentedLines = []string{dedentCodeLine(line)}
			i++
			continue
		}

		currentLines = append(currentLines, line)
		i++
	}

	if inIndented {
		closeIndentedBlock()
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

// atxLevel cuenta los '#' iniciales de un encabezado ATX ya confirmado por
// markdownHeaderTitle.
func atxLevel(trimmed string) int {
	count := 0
	for _, ch := range trimmed {
		if ch == '#' {
			count++
		} else {
			break
		}
	}
	return count
}

// scanFencedBlock busca, desde startIdx en adelante, la extensión real de un
// bloque fenced abierto con el delimitador (ch, minLen) dentro de una
// sección de nivel openerLevel.
//
// Primero busca un cierre real (matchFenceClose) en TODO el resto del
// documento. Si existe, el bloque es uno normal y cerrado: su contenido es
// literal de principio a fin, sin importar que alguna línea adentro parezca
// un encabezado (así CASO 1 y CASO 2 del documento adversarial, con
// "# comentarios" y encabezados falsos dentro de bloques que sí cierran,
// quedan intactos).
//
// Solo si NINGÚN cierre real existe en el resto del archivo se considera
// "sin cerrar", y ahí se aplica la recuperación: se busca el primer
// encabezado de nivel igual o superior (<=) al de la sección donde abrió el
// bloque, y ese es el límite donde se asume que terminó implícitamente. Si
// tampoco hay un encabezado así, el bloque se extiende hasta el final del
// archivo.
//
// Retorna las líneas de contenido, el índice donde debe continuar la
// segmentación (el de la línea de cierre real si closed=true, o el del
// encabezado de recuperación / len(lines) si closed=false), si se encontró
// un cierre real, y la línea (1-indexada) del encabezado de recuperación
// usado (0 si no aplicó ninguno).
func scanFencedBlock(lines []string, startIdx int, ch rune, minLen int, openerLevel int) (content []string, endIdx int, closed bool, recoveredAtLine int) {
	for j := startIdx; j < len(lines); j++ {
		if matchFenceClose(strings.TrimSpace(lines[j]), ch, minLen) {
			return append([]string{}, lines[startIdx:j]...), j, true, 0
		}
	}

	for j := startIdx; j < len(lines); j++ {
		trimmed := strings.TrimSpace(lines[j])
		if isHeader, _ := markdownHeaderTitle(trimmed); isHeader {
			if level := atxLevel(trimmed); level > 0 && level <= openerLevel {
				return append([]string{}, lines[startIdx:j]...), j, false, j + 1
			}
		}
	}

	return append([]string{}, lines[startIdx:]...), len(lines), false, 0
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
