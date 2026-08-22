package converter

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// listIndent es la indentación usada por nivel de anidamiento en listas
// Markdown. Se usan 4 espacios en vez de 2 porque es el ancho que la mayoría
// de renderizadores Markdown (incluido CommonMark) requiere para reconocer de
// forma confiable un ítem anidado bajo un marcador ordenado ("1. ", 3-4
// caracteres); con 2 espacios varios renderizadores lo tratan como texto
// suelto en vez de como sub-lista.
const listIndent = "    "

// ParseHTML convierte un documento HTML en unidades lógicas dividiendo por encabezados.
// Los encabezados h1, h2 y h3 inician nuevas unidades. Las tablas y listas se
// convierten a su sintaxis Markdown equivalente en vez de aplanarse a texto plano.
func ParseHTML(content string) []Unit {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		// Si falla el parsing, tratar como texto plano
		return ParsePlainText(content)
	}

	var segments []struct {
		title string
		lines []string
		notes []string
	}

	currentTitle := ""
	var currentLines []string
	var currentNotes []string
	idx := 0

	flush := func() {
		if len(currentLines) > 0 || currentTitle != "" {
			segments = append(segments, struct {
				title string
				lines []string
				notes []string
			}{currentTitle, currentLines, currentNotes})
			currentLines = nil
			currentNotes = nil
			idx++
		}
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			switch tag {
			case "h1", "h2", "h3":
				flush()
				currentTitle = extractText(n)
				return
			case "script", "style", "head":
				// Ignorar elementos no de contenido
				return
			case "table":
				lines, note := renderTable(n)
				if len(lines) > 0 {
					currentLines = append(currentLines, lines...)
					currentLines = append(currentLines, "")
					currentNotes = append(currentNotes, note)
				}
				return
			case "ul", "ol":
				lines, count := renderList(n, 0)
				if len(lines) > 0 {
					currentLines = append(currentLines, lines...)
					currentLines = append(currentLines, "")
					currentNotes = append(currentNotes, fmt.Sprintf("Lista <%s> convertida a Markdown: %d ítems", tag, count))
				}
				return
			case "p", "li", "td", "th":
				text := strings.TrimSpace(extractText(n))
				if text != "" {
					currentLines = append(currentLines, text)
					currentLines = append(currentLines, "")
				}
				return
			case "pre", "code":
				text := extractText(n)
				currentLines = append(currentLines, "```")
				currentLines = append(currentLines, text)
				currentLines = append(currentLines, "```")
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	walk(doc)
	flush()

	if len(segments) == 0 {
		return []Unit{{Title: "Documento", Content: extractAllText(doc), Slug: "documento.md"}}
	}

	var units []Unit
	for i, seg := range segments {
		title := seg.title
		slug, fallback := makeSlug(i+1, title)
		units = append(units, Unit{
			Title:            title,
			Content:          strings.TrimSpace(strings.Join(seg.lines, "\n")),
			Slug:             slug,
			FallbackSlugUsed: fallback,
			Notes:            seg.notes,
		})
	}

	return units
}

// extractText extrae el texto plano de un nodo HTML.
func extractText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// extractAllText extrae todo el texto visible de un documento HTML.
func extractAllText(doc *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "script" || tag == "style" || tag == "head" {
				return
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(text)
				sb.WriteString("\n")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return sb.String()
}

// ─── Tablas ──────────────────────────────────────────────────────────────────

// renderTable convierte un elemento <table> en una tabla Markdown con sintaxis
// de pipes, preservando el número de columnas de cada fila — las celdas
// vacías se conservan como celdas vacías, no se omiten ni colapsan la
// columna. Si la tabla no tiene <thead>, se usa la primera fila de datos como
// fila de encabezado en lugar de inventar nombres de columna genéricos: así
// no se pierde esa fila del contenido original, solo se reclasifica.
// Retorna las líneas Markdown de la tabla y una nota para log.md con el
// tamaño detectado y el origen del encabezado.
func renderTable(n *html.Node) ([]string, string) {
	var headerCells []*html.Node
	headerFromFirstRow := false

	var bodyRows []*html.Node
	if tbody := findChild(n, "tbody"); tbody != nil {
		bodyRows = findChildren(tbody, "tr")
	} else {
		bodyRows = findChildren(n, "tr")
	}

	if thead := findChild(n, "thead"); thead != nil {
		if headRows := findChildren(thead, "tr"); len(headRows) > 0 {
			headerCells = findCells(headRows[0])
		}
	}

	if headerCells == nil {
		if len(bodyRows) == 0 {
			return nil, ""
		}
		headerCells = findCells(bodyRows[0])
		bodyRows = bodyRows[1:]
		headerFromFirstRow = true
	}

	numCols := len(headerCells)
	for _, row := range bodyRows {
		if c := len(findCells(row)); c > numCols {
			numCols = c
		}
	}
	if numCols == 0 {
		return nil, ""
	}

	padRow := func(cells []*html.Node) []string {
		row := make([]string, numCols)
		for i := 0; i < numCols && i < len(cells); i++ {
			row[i] = tableCellText(cells[i])
		}
		return row
	}

	var lines []string
	lines = append(lines, "| "+strings.Join(padRow(headerCells), " | ")+" |")

	sep := make([]string, numCols)
	for i := range sep {
		sep[i] = "---"
	}
	lines = append(lines, "| "+strings.Join(sep, " | ")+" |")

	for _, row := range bodyRows {
		lines = append(lines, "| "+strings.Join(padRow(findCells(row)), " | ")+" |")
	}

	headerNote := "encabezado desde <thead>"
	if headerFromFirstRow {
		headerNote = "sin <thead>: se usó la primera fila como encabezado"
	}
	note := fmt.Sprintf("Tabla convertida a Markdown: %d columnas x %d filas de datos (%s)", numCols, len(bodyRows), headerNote)

	return lines, note
}

// tableCellText normaliza el texto de una celda para que quepa en una sola
// línea de tabla Markdown: colapsa saltos de línea/espacios internos y
// escapa los pipes literales para no romper la sintaxis de la tabla.
func tableCellText(n *html.Node) string {
	text := strings.Join(strings.Fields(extractText(n)), " ")
	return strings.ReplaceAll(text, "|", "\\|")
}

// findChild retorna el primer hijo directo de n con la etiqueta dada.
func findChild(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == tag {
			return c
		}
	}
	return nil
}

// findChildren retorna todos los hijos directos de n con la etiqueta dada.
func findChildren(n *html.Node, tag string) []*html.Node {
	var result []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.ToLower(c.Data) == tag {
			result = append(result, c)
		}
	}
	return result
}

// findCells retorna los hijos <td>/<th> directos de una fila <tr>.
func findCells(tr *html.Node) []*html.Node {
	var result []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			tag := strings.ToLower(c.Data)
			if tag == "td" || tag == "th" {
				result = append(result, c)
			}
		}
	}
	return result
}

// ─── Listas ──────────────────────────────────────────────────────────────────

// renderList convierte un <ul>/<ol> en líneas Markdown, soportando listas
// anidadas (un <ul>/<ol> dentro de un <li>) mediante recursión con
// indentación creciente por nivel. Retorna las líneas y la cantidad de
// ítems de ESTE nivel de lista (no cuenta los de sub-listas anidadas, que
// generan su propia nota de log al ser procesadas).
func renderList(n *html.Node, depth int) ([]string, int) {
	ordered := strings.ToLower(n.Data) == "ol"
	indent := strings.Repeat(listIndent, depth)

	var lines []string
	count := 0

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || strings.ToLower(c.Data) != "li" {
			continue
		}
		count++

		marker := "- "
		if ordered {
			marker = fmt.Sprintf("%d. ", count)
		}
		text := strings.Join(strings.Fields(extractListItemText(c)), " ")
		lines = append(lines, indent+marker+text)

		for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
			if gc.Type == html.ElementNode {
				gtag := strings.ToLower(gc.Data)
				if gtag == "ul" || gtag == "ol" {
					nestedLines, _ := renderList(gc, depth+1)
					lines = append(lines, nestedLines...)
				}
			}
		}
	}

	return lines, count
}

// extractListItemText extrae el texto propio de un <li>, excluyendo el
// contenido de cualquier <ul>/<ol> anidado (esas sub-listas se renderizan
// por separado en renderList para no duplicar su texto como una sola línea).
func extractListItemText(li *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag := strings.ToLower(node.Data)
			if tag == "ul" || tag == "ol" {
				return
			}
		}
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for c := li.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	return sb.String()
}
