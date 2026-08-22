package converter

import (
	"strings"
	"testing"
)

func TestParseHTML_TableBecomesMarkdownTable(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Título</h1>
		<table>
			<thead><tr><th>Columna 1</th><th>Columna 2</th></tr></thead>
			<tbody>
				<tr><td>occaecat-0-0</td><td>occaecat-0-1</td></tr>
				<tr><td>proident-1-0</td><td>dolor-1-1</td></tr>
			</tbody>
		</table>
	</body></html>`

	units := ParseHTML(htmlDoc)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad, hubo %d", len(units))
	}
	content := units[0].Content

	lines := nonEmptyLines(content)
	// encabezado + separador + 2 filas de datos = 4 líneas de tabla
	tableLines := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "|") {
			tableLines++
			// 2 columnas → 3 pipes por línea ("| a | b |")
			if strings.Count(l, "|") != 3 {
				t.Errorf("línea de tabla con número de columnas incorrecto: %q", l)
			}
		}
	}
	if tableLines != 4 {
		t.Fatalf("se esperaban 4 líneas de tabla (encabezado+separador+2 filas), hubo %d:\n%s", tableLines, content)
	}
	if !strings.Contains(content, "| Columna 1 | Columna 2 |") {
		t.Errorf("no se encontró la fila de encabezado esperada:\n%s", content)
	}
	if !strings.Contains(content, "| --- | --- |") {
		t.Errorf("no se encontró la fila separadora esperada:\n%s", content)
	}
	if !strings.Contains(content, "| occaecat-0-0 | occaecat-0-1 |") {
		t.Errorf("no se encontró la primera fila de datos esperada:\n%s", content)
	}
}

func TestParseHTML_UnorderedListItemsGetDashPrefix(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Título</h1>
		<ul>
			<li>Aute item 0</li>
			<li>Labore item 1</li>
			<li>Eu item 2</li>
		</ul>
	</body></html>`

	units := ParseHTML(htmlDoc)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad, hubo %d", len(units))
	}

	dashLines := 0
	for _, l := range nonEmptyLines(units[0].Content) {
		if strings.HasPrefix(l, "- ") {
			dashLines++
		}
	}
	if dashLines != 3 {
		t.Errorf("se esperaban 3 líneas con prefijo '- ', hubo %d:\n%s", dashLines, units[0].Content)
	}
}

func TestParseHTML_OrderedListItemsAreNumberedInOrder(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Título</h1>
		<ol>
			<li>Primero</li>
			<li>Segundo</li>
			<li>Tercero</li>
			<li>Cuarto</li>
		</ol>
	</body></html>`

	units := ParseHTML(htmlDoc)
	var numbered []string
	for _, l := range nonEmptyLines(units[0].Content) {
		for _, prefix := range []string{"1. ", "2. ", "3. ", "4. "} {
			if strings.HasPrefix(l, prefix) {
				numbered = append(numbered, l)
			}
		}
	}
	if len(numbered) != 4 {
		t.Fatalf("se esperaban 4 líneas numeradas, hubo %d:\n%s", len(numbered), units[0].Content)
	}
	for i, l := range numbered {
		expectedPrefix := []string{"1. ", "2. ", "3. ", "4. "}[i]
		if !strings.HasPrefix(l, expectedPrefix) {
			t.Errorf("línea %d = %q, se esperaba el prefijo %q en orden ascendente sin saltos", i, l, expectedPrefix)
		}
	}
}

func TestParseHTML_NestedListIndentsSubItems(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Título</h1>
		<ul>
			<li>Padre 1
				<ul>
					<li>Hijo 1.1</li>
					<li>Hijo 1.2</li>
				</ul>
			</li>
			<li>Padre 2</li>
		</ul>
	</body></html>`

	units := ParseHTML(htmlDoc)
	content := units[0].Content

	if !strings.Contains(content, "- Padre 1") {
		t.Errorf("falta el ítem padre:\n%s", content)
	}
	if !strings.Contains(content, listIndent+"- Hijo 1.1") {
		t.Errorf("el hijo anidado no tiene la indentación esperada (%q):\n%s", listIndent, content)
	}
	if !strings.Contains(content, listIndent+"- Hijo 1.2") {
		t.Errorf("el segundo hijo anidado no tiene la indentación esperada:\n%s", content)
	}
	if !strings.Contains(content, "- Padre 2") {
		t.Errorf("falta el segundo ítem padre:\n%s", content)
	}
}

func TestParseHTML_EmptyCellKeepsColumnCount(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Título</h1>
		<table>
			<thead><tr><th>A</th><th>B</th><th>C</th></tr></thead>
			<tbody>
				<tr><td>uno</td><td></td><td>tres</td></tr>
			</tbody>
		</table>
	</body></html>`

	units := ParseHTML(htmlDoc)
	content := units[0].Content

	var dataRow string
	for _, l := range nonEmptyLines(content) {
		if strings.HasPrefix(l, "| uno") {
			dataRow = l
		}
	}
	if dataRow == "" {
		t.Fatalf("no se encontró la fila de datos con la celda vacía:\n%s", content)
	}
	if strings.Count(dataRow, "|") != 4 {
		t.Errorf("la celda vacía hizo desaparecer una columna: %q (se esperaban 4 pipes para 3 columnas)", dataRow)
	}
	if !strings.Contains(dataRow, "| uno |  | tres |") {
		t.Errorf("la celda vacía no se preservó como celda vacía: %q", dataRow)
	}
}

func TestParseHTML_TableWithoutTheadUsesFirstRowAsHeader(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Título</h1>
		<table>
			<tr><td>Encabezado A</td><td>Encabezado B</td></tr>
			<tr><td>dato-1</td><td>dato-2</td></tr>
		</table>
	</body></html>`

	units := ParseHTML(htmlDoc)
	content := units[0].Content

	if !strings.Contains(content, "| Encabezado A | Encabezado B |") {
		t.Errorf("la primera fila no se usó como encabezado:\n%s", content)
	}
	if !strings.Contains(units[0].Notes[len(units[0].Notes)-1], "sin <thead>") {
		t.Errorf("la nota de log no documenta que se usó la primera fila como encabezado: %v", units[0].Notes)
	}
}

func TestParseHTML_LogsNoteForEachTableAndList(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Título</h1>
		<table>
			<thead><tr><th>A</th><th>B</th></tr></thead>
			<tbody><tr><td>1</td><td>2</td></tr></tbody>
		</table>
		<ul><li>x</li><li>y</li></ul>
	</body></html>`

	units := ParseHTML(htmlDoc)
	if len(units[0].Notes) != 2 {
		t.Fatalf("se esperaban 2 notas (1 tabla + 1 lista), hubo %d: %v", len(units[0].Notes), units[0].Notes)
	}
	if !strings.Contains(units[0].Notes[0], "Tabla convertida") {
		t.Errorf("primera nota inesperada: %q", units[0].Notes[0])
	}
	if !strings.Contains(units[0].Notes[1], "Lista <ul> convertida") {
		t.Errorf("segunda nota inesperada: %q", units[0].Notes[1])
	}
}

// Ruta feliz: un documento sin tablas ni listas no debe cambiar su salida.
func TestParseHTML_PlainContentUnaffected(t *testing.T) {
	htmlDoc := `<html><body>
		<h1>Introducción</h1>
		<p>Primer párrafo.</p>
		<p>Segundo párrafo.</p>
		<h2>Sección</h2>
		<p>Contenido de la sección.</p>
	</body></html>`

	units := ParseHTML(htmlDoc)
	if len(units) != 2 {
		t.Fatalf("se esperaban 2 unidades, hubo %d", len(units))
	}
	if units[0].Title != "Introducción" {
		t.Errorf("título = %q, se esperaba 'Introducción'", units[0].Title)
	}
	if !strings.Contains(units[0].Content, "Primer párrafo.") || !strings.Contains(units[0].Content, "Segundo párrafo.") {
		t.Errorf("contenido de párrafos ausente: %q", units[0].Content)
	}
	if len(units[0].Notes) != 0 {
		t.Errorf("no debería haber notas sin tablas/listas: %v", units[0].Notes)
	}
	if units[1].Title != "Sección" {
		t.Errorf("título = %q, se esperaba 'Sección'", units[1].Title)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
