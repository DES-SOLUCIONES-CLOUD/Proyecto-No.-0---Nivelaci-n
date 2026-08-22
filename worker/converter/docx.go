package converter

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ─── Estructuras XML de DOCX ─────────────────────────────────────────────────

// docxBody representa el cuerpo del documento XML de Word.
type docxBody struct {
	Paragraphs []docxParagraph `xml:"body>p"`
}

// docxParagraph representa un párrafo en el documento Word.
type docxParagraph struct {
	Props docxParaProps `xml:"pPr"`
	Runs  []docxRun     `xml:"r"`
}

// docxParaProps contiene las propiedades del párrafo (incluyendo el estilo).
type docxParaProps struct {
	Style docxParaStyle `xml:"pStyle"`
}

// docxParaStyle referencia el estilo aplicado al párrafo.
type docxParaStyle struct {
	Val string `xml:"val,attr"`
}

// docxRun representa una ejecución de texto dentro de un párrafo.
type docxRun struct {
	Text docxText `xml:"t"`
}

// docxText contiene el texto real del run.
type docxText struct {
	Value string `xml:",chardata"`
}

// ─── Parser principal ─────────────────────────────────────────────────────────

// ParseDOCX extrae el contenido de un archivo .docx (ZIP de XML) y lo divide
// en unidades lógicas por encabezados (Heading1, Heading2).
// Si no hay encabezados, todo el contenido va a una sola unidad.
func ParseDOCX(data []byte) []Unit {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fallbackUnit("No se pudo leer el archivo DOCX.")
	}

	// Buscar word/document.xml dentro del ZIP
	var docXML io.ReadCloser
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docXML, err = f.Open()
			if err != nil {
				return fallbackUnit("No se pudo abrir word/document.xml.")
			}
			defer docXML.Close()
			break
		}
	}

	if docXML == nil {
		return fallbackUnit("El archivo no contiene word/document.xml (¿es un .docx válido?).")
	}

	// Parsear el XML
	raw, err := io.ReadAll(docXML)
	if err != nil {
		return fallbackUnit("Error leyendo el XML del documento.")
	}

	var body docxBody
	// El namespace del documento Word es largo; usamos un decodificador que lo ignora
	decoder := xml.NewDecoder(bytes.NewReader(stripNamespaces(raw)))
	if err := decoder.Decode(&body); err != nil {
		// Fallback: extraer texto plano del XML sin parsear la estructura
		plainText := extractPlainTextFromXML(raw)
		if plainText == "" {
			return fallbackUnit("No se pudo extraer texto del documento.")
		}
		return ParsePlainText(plainText)
	}

	return buildUnitsFromDocx(body.Paragraphs)
}

// buildUnitsFromDocx construye las unidades lógicas a partir de los párrafos DOCX.
func buildUnitsFromDocx(paragraphs []docxParagraph) []Unit {
	var units []Unit
	var currentTitle string
	var currentLines []string
	unitIdx := 0

	isHeading := func(style string) bool {
		s := strings.ToLower(style)
		return strings.HasPrefix(s, "heading") ||
			strings.HasPrefix(s, "titulo") ||
			strings.HasPrefix(s, "ttulo") ||
			s == "h1" || s == "h2" || s == "h3"
	}

	flush := func() {
		body := strings.TrimSpace(strings.Join(currentLines, "\n"))
		title := currentTitle
		if title == "" && body == "" {
			return
		}
		if title == "" {
			title = "Documento"
		}
		slug, fallback := makeSlug(unitIdx+1, title)
		units = append(units, Unit{
			Title:            title,
			Content:          body,
			Slug:             slug,
			FallbackSlugUsed: fallback,
		})
		unitIdx++
		currentLines = nil
		currentTitle = ""
	}

	for _, p := range paragraphs {
		// Extraer texto del párrafo (concatenar todos sus runs)
		var textParts []string
		for _, run := range p.Runs {
			t := strings.TrimSpace(run.Text.Value)
			if t != "" {
				textParts = append(textParts, t)
			}
		}
		text := strings.Join(textParts, " ")

		style := p.Props.Style.Val
		if isHeading(style) && text != "" {
			flush()
			currentTitle = text
		} else if text != "" {
			currentLines = append(currentLines, text)
		}
	}
	flush()

	if len(units) == 0 {
		return fallbackUnit("El documento no contiene texto.")
	}
	return units
}

// fallbackUnit retorna una unidad única con un mensaje de error o advertencia.
func fallbackUnit(msg string) []Unit {
	return []Unit{
		{
			Title:   "Documento",
			Content: fmt.Sprintf("*%s*", msg),
			Slug:    "concepto-01-documento.md",
		},
	}
}

// stripNamespaces reemplaza los prefijos de namespace XML de Word para facilitar
// el parsing con el decoder estándar de Go (que es estricto con namespaces).
func stripNamespaces(data []byte) []byte {
	s := string(data)
	// Remover declaraciones xmlns y prefijos w: w14: r: etc.
	replacements := []struct{ old, new string }{
		{"<w:", "<"}, {"</w:", "</"}, {" w:", " "},
		{"<r>", "<r>"}, {"</r>", "</r>"},
		{"<wp:", "<"}, {"</wp:", "</"},
	}
	for _, rep := range replacements {
		s = strings.ReplaceAll(s, rep.old, rep.new)
	}
	return []byte(s)
}

// extractPlainTextFromXML extrae el texto de los tags <w:t> de forma simple,
// sin depender del parser XML completo. Usado como fallback.
func extractPlainTextFromXML(data []byte) string {
	s := string(data)
	var sb strings.Builder
	for {
		start := strings.Index(s, "<w:t")
		if start == -1 {
			break
		}
		// Avanzar hasta el cierre del tag de apertura
		close := strings.Index(s[start:], ">")
		if close == -1 {
			break
		}
		s = s[start+close+1:]
		end := strings.Index(s, "</w:t>")
		if end == -1 {
			break
		}
		text := strings.TrimSpace(s[:end])
		if text != "" {
			sb.WriteString(text)
			sb.WriteString(" ")
		}
		s = s[end+6:]
	}
	return strings.TrimSpace(sb.String())
}
