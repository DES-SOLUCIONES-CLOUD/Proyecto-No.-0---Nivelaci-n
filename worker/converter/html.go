package converter

import (
	"strings"

	"golang.org/x/net/html"
)

// ParseHTML convierte un documento HTML en unidades lógicas dividiendo por encabezados.
// Los encabezados h1, h2 y h3 inician nuevas unidades.
func ParseHTML(content string) []Unit {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		// Si falla el parsing, tratar como texto plano
		return ParsePlainText(content)
	}

	var segments []struct {
		title string
		lines []string
	}

	currentTitle := ""
	var currentLines []string
	idx := 0

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			switch tag {
			case "h1", "h2", "h3":
				// Flush segmento anterior
				if len(currentLines) > 0 || currentTitle != "" {
					segments = append(segments, struct {
						title string
						lines []string
					}{currentTitle, currentLines})
					currentLines = nil
					idx++
				}
				currentTitle = extractText(n)
				return
			case "script", "style", "head":
				// Ignorar elementos no de contenido
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

	// Flush último segmento
	if len(currentLines) > 0 || currentTitle != "" {
		segments = append(segments, struct {
			title string
			lines []string
		}{currentTitle, currentLines})
	}

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
