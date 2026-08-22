package converter

import (
	"fmt"
	"regexp"
	"strings"
)

// orderedListItemTxtRegex reconoce un ítem de lista ordenada en texto plano
// ("1. texto", "2. texto", …), sin importar cuánta indentación tenga delante
// (se evalúa siempre sobre la línea ya recortada).
var orderedListItemTxtRegex = regexp.MustCompile(`^\d+\.\s`)

// ParsePlainText divide un documento de texto plano en unidades lógicas.
//
// Los encabezados se reconocen en dos convenciones:
//  1. Markdown ("# Título", "## Título", …), para documentos .txt que ya usan esa sintaxis.
//  2. Setext: una línea de texto sin indentar seguida inmediatamente de una
//     línea compuesta solo de '=' (nivel 1) o de '-' (nivel 2), de longitud
//     al menos la mitad de la del texto que subraya. Una línea de guiones
//     precedida por una línea en blanco (p. ej. un separador decorativo
//     "----------------------------------------" entre secciones) nunca
//     cuenta como subrayado, porque el chequeo de encabezado solo se activa
//     cuando la línea ANTERIOR a los guiones tiene texto real.
//
// Dentro de un bloque de código indentado (4 espacios o un tab, precedido de
// línea en blanco) ninguna línea se evalúa como encabezado ni como límite de
// sección, sin importar con qué carácter empiece: así un comentario "# ..."
// de Python dentro del bloque no se confunde con un encabezado, y el bloque
// nunca queda partido entre dos conceptos. El bloque se envuelve en ``` al
// convertirlo a Markdown.
//
// Si una unidad lógica queda sin ningún encabezado real asociado (por
// ejemplo el preámbulo antes del primer encabezado), se usa un título
// genérico explícito ("Sección sin título N") y se registra una advertencia
// en el log — nunca se infiere un título a partir de una oración de prosa.
func ParsePlainText(content string) []Unit {
	lines := strings.Split(content, "\n")

	var units []Unit
	var currentTitle string
	var currentLines []string
	var currentNotes []string
	unitIdx := 0

	flush := func() {
		body := strings.TrimSpace(strings.Join(currentLines, "\n"))
		title := currentTitle
		if title == "" && body == "" {
			currentLines = nil
			currentNotes = nil
			return
		}
		if title == "" {
			if unitIdx == 0 {
				// Documento sin ningún encabezado real desde el principio:
				// no es una unidad "huérfana" dentro de una estructura, es
				// la convención ya establecida para un documento sin
				// estructura (igual que ParseMarkdown y los demás
				// conversores), así que no amerita advertencia.
				title = "Documento"
			} else {
				title = fmt.Sprintf("Sección sin título %d", unitIdx+1)
				currentNotes = append(currentNotes, fmt.Sprintf("No se detectó un encabezado real para esta unidad; se usó el título genérico '%s'", title))
			}
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

	inCodeBlock := false
	var codeLines []string

	closeCodeBlock := func() {
		for len(codeLines) > 0 && codeLines[len(codeLines)-1] == "" {
			codeLines = codeLines[:len(codeLines)-1]
		}
		if len(codeLines) > 0 {
			currentLines = append(currentLines, "```")
			currentLines = append(currentLines, codeLines...)
			currentLines = append(currentLines, "```", "")
			currentNotes = append(currentNotes, fmt.Sprintf("Bloque de código indentado convertido a Markdown (%d líneas)", len(codeLines)))
		}
		codeLines = nil
		inCodeBlock = false
	}

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if inCodeBlock {
			if trimmed == "" {
				codeLines = append(codeLines, "")
				i++
				continue
			}
			if isIndentedCodeLine(line) {
				codeLines = append(codeLines, dedentCodeLine(line))
				i++
				continue
			}
			closeCodeBlock()
			continue // reprocesar esta línea: ya no es parte del bloque
		}

		if trimmed == "" {
			currentLines = append(currentLines, line)
			i++
			continue
		}

		isUnindented := !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t")

		// Encabezado setext (solo sobre líneas sin indentar: un encabezado
		// real siempre arranca en la columna 0 en este documento).
		if isUnindented && setextLevel(lines, i) > 0 {
			flush()
			currentTitle = trimmed
			i += 2
			continue
		}

		// Encabezado Markdown ("#", "##", …)
		if isUnindented {
			if isHeader, title := markdownHeaderTitle(trimmed); isHeader {
				flush()
				currentTitle = title
				i++
				continue
			}
		}

		// Tabla ASCII con bordes "+---+---+"
		if isTableBorderLine(trimmed) {
			mdLines, next, cols, dataRows := consumeASCIITable(lines, i)
			if len(mdLines) > 0 {
				currentLines = append(currentLines, mdLines...)
				currentLines = append(currentLines, "")
				currentNotes = append(currentNotes, fmt.Sprintf("Tabla ASCII convertida a Markdown: %d columnas x %d filas de datos", cols, dataRows))
			}
			i = next
			continue
		}

		// Cita indentada ("    > texto") → blockquote Markdown ("> texto")
		if strings.HasPrefix(trimmed, "> ") {
			currentLines = append(currentLines, trimmed)
			i++
			continue
		}

		// Lista numerada o con viñetas, indentada — la sintaxis ("N. "/"- ")
		// ya es Markdown válido, solo hace falta quitar la indentación.
		if orderedListItemTxtRegex.MatchString(trimmed) || strings.HasPrefix(trimmed, "- ") {
			currentLines = append(currentLines, trimmed)
			i++
			continue
		}

		// Bloque de código indentado: arranca solo si la línea anterior fue
		// una línea en blanco (evita confundir con listas/citas, ya
		// descartadas arriba, o con texto casualmente indentado a mano).
		if isIndentedCodeLine(line) && (i == 0 || strings.TrimSpace(lines[i-1]) == "") {
			inCodeBlock = true
			codeLines = []string{dedentCodeLine(line)}
			i++
			continue
		}

		currentLines = append(currentLines, line)
		i++
	}

	if inCodeBlock {
		closeCodeBlock()
	}
	flush()

	if len(units) == 0 {
		units = []Unit{{
			Title:            "Documento",
			Content:          strings.TrimSpace(content),
			Slug:             "concepto-01-documento.md",
			FallbackSlugUsed: false,
		}}
	}

	return units
}

// setextLevel retorna 1 si lines[i] es el texto de un encabezado subrayado
// con '=', 2 si lo es con '-', o 0 si lines[i] no es un encabezado setext.
// El subrayado debe tener longitud >= 3 y al menos la mitad de la longitud
// del texto que subraya; de lo contrario se descarta como falso positivo
// (p. ej. un separador de guiones muy corto o muy largo frente al texto).
func setextLevel(lines []string, i int) int {
	if i+1 >= len(lines) {
		return 0
	}
	text := strings.TrimSpace(lines[i])
	underline := strings.TrimSpace(lines[i+1])
	if text == "" || len(underline) < 3 {
		return 0
	}

	var level int
	switch {
	case isAllRune(underline, '='):
		level = 1
	case isAllRune(underline, '-'):
		level = 2
	default:
		return 0
	}

	if len(underline) < len(text)/2 {
		return 0
	}
	return level
}

// isAllRune verifica que s esté compuesta únicamente por el carácter r.
func isAllRune(s string, r rune) bool {
	for _, ch := range s {
		if ch != r {
			return false
		}
	}
	return true
}

// markdownHeaderTitle reconoce un encabezado Markdown ("#", "##", …) y
// retorna su texto sin el prefijo.
func markdownHeaderTitle(trimmed string) (bool, string) {
	if !strings.HasPrefix(trimmed, "#") {
		return false, ""
	}
	hashCount := 0
	for _, ch := range trimmed {
		if ch == '#' {
			hashCount++
		} else {
			break
		}
	}
	if hashCount > 0 && len(trimmed) > hashCount && trimmed[hashCount] == ' ' {
		return true, strings.TrimSpace(trimmed[hashCount:])
	}
	return false, ""
}

// ─── Bloques de código indentados ───────────────────────────────────────────

// isIndentedCodeLine reconoce una línea indentada con 4+ espacios o un tab.
func isIndentedCodeLine(line string) bool {
	return strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")
}

// dedentCodeLine quita un nivel de indentación (4 espacios o un tab) de una
// línea de código, preservando la indentación interna del código (p. ej. el
// cuerpo de una función dentro del bloque).
func dedentCodeLine(line string) string {
	if strings.HasPrefix(line, "\t") {
		return strings.TrimPrefix(line, "\t")
	}
	return strings.TrimPrefix(line, "    ")
}

// ─── Tablas ASCII ────────────────────────────────────────────────────────────

// isTableBorderLine reconoce una línea de borde de tabla ASCII: "+---+---+".
func isTableBorderLine(trimmed string) bool {
	if len(trimmed) < 3 || trimmed[0] != '+' || trimmed[len(trimmed)-1] != '+' {
		return false
	}
	for _, ch := range trimmed {
		if ch != '+' && ch != '-' {
			return false
		}
	}
	return true
}

// isPipeRowLine reconoce una fila de tabla ASCII: "| celda | celda |".
func isPipeRowLine(trimmed string) bool {
	return len(trimmed) >= 2 && strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

// splitTableCells separa una fila "| a | b |" en sus celdas, recortando el
// relleno de espacios usado para alinear las columnas en ASCII.
func splitTableCells(row string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(row, "|"), "|")
	parts := strings.Split(inner, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// consumeASCIITable consume, a partir de lines[i] (una línea de borde), todas
// las líneas de una tabla ASCII (bordes y filas "| ... |") y las convierte a
// una tabla Markdown de pipes. La primera fila de datos se usa como
// encabezado, igual que hace el conversor HTML cuando no hay <thead>: así no
// se pierde esa fila del contenido original. Retorna las líneas Markdown, el
// índice siguiente a procesar, y el número de columnas y filas de datos
// detectadas (para la nota de log).
func consumeASCIITable(lines []string, i int) ([]string, int, int, int) {
	var rows [][]string
	j := i
	for j < len(lines) {
		trimmed := strings.TrimSpace(lines[j])
		if isTableBorderLine(trimmed) {
			j++
			continue
		}
		if isPipeRowLine(trimmed) {
			rows = append(rows, splitTableCells(trimmed))
			j++
			continue
		}
		break
	}

	if len(rows) == 0 {
		return nil, i + 1, 0, 0
	}

	numCols := 0
	for _, r := range rows {
		if len(r) > numCols {
			numCols = len(r)
		}
	}

	pad := func(r []string) []string {
		out := make([]string, numCols)
		copy(out, r)
		return out
	}

	var md []string
	md = append(md, "| "+strings.Join(pad(rows[0]), " | ")+" |")

	sep := make([]string, numCols)
	for k := range sep {
		sep[k] = "---"
	}
	md = append(md, "| "+strings.Join(sep, " | ")+" |")

	for _, r := range rows[1:] {
		md = append(md, "| "+strings.Join(pad(r), " | ")+" |")
	}

	return md, j, numCols, len(rows) - 1
}
