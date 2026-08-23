package converter

import (
	"strings"
	"testing"
)

func TestMakeSlug(t *testing.T) {
	tests := []struct {
		name             string
		title            string
		expectedSlug     string
		expectedFallback bool
	}{
		{
			name:             "Título normal",
			title:            "Este es un título normal",
			expectedSlug:     "concepto-01-este-es-un-titulo-normal.md",
			expectedFallback: false,
		},
		{
			name:             "Con símbolos",
			title:            "1. ¿Qué es Design Thinking?",
			expectedSlug:     "concepto-02-1-que-es-design-thinking.md",
			expectedFallback: false,
		},
		{
			name:             "Mojibake",
			title:            "'HVLJQ7KLQNLQJ:RUNVKRS",
			expectedSlug:     "concepto-03-hvljq7klqnlqj-runvkrs.md",
			expectedFallback: false, // Tiene > 3 letras, así que no usará fallback, pero los símbolos se limpian.
		},
		{
			name:             "Solo símbolos (fallback)",
			title:            "!@#$%^&*",
			expectedSlug:     "concepto-04-concepto.md",
			expectedFallback: true,
		},
		{
			name:             "Pocas letras (fallback)",
			title:            "A B",
			expectedSlug:     "concepto-05-concepto.md",
			expectedFallback: true,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSlug, gotFallback := makeSlug(i+1, tt.title)
			if gotSlug != tt.expectedSlug {
				t.Errorf("makeSlug() slug = %v, se esperaba %v", gotSlug, tt.expectedSlug)
			}
			if gotFallback != tt.expectedFallback {
				t.Errorf("makeSlug() fallback = %v, se esperaba %v", gotFallback, tt.expectedFallback)
			}
		})
	}
}

func TestParseMarkdown_HashCommentInsideFencedBlockIsNotAHeading(t *testing.T) {
	for _, lang := range []string{"python", "bash", "javascript", "yaml", ""} {
		t.Run("lang="+lang, func(t *testing.T) {
			doc := "## Sección\n\n" +
				"Texto de la sección.\n\n" +
				"### Ejemplo de código\n\n" +
				"```" + lang + "\n" +
				"def funcion(x, y):\n" +
				"    # Comentario explicativo aquí\n" +
				"    resultado = x + y\n" +
				"    return resultado\n" +
				"```\n\n" +
				"Más texto después del bloque de código.\n"

			units := ParseMarkdown(doc)

			// El bloque de código debe quedar dentro de UN solo concepto: el
			// creado por el encabezado "### Ejemplo de código".
			var codeUnit *Unit
			for i := range units {
				if units[i].Title == "Ejemplo de código" {
					codeUnit = &units[i]
				}
			}
			if codeUnit == nil {
				t.Fatalf("no se encontró el concepto 'Ejemplo de código' entre %d unidades: %+v", len(units), titles(units))
			}

			fenceCount := strings.Count(codeUnit.Content, "```")
			if fenceCount != 2 {
				t.Errorf("se esperaban exactamente 2 líneas ``` (apertura+cierre) en el concepto, hubo %d:\n%s", fenceCount, codeUnit.Content)
			}
			if !strings.Contains(codeUnit.Content, "# Comentario explicativo aquí") {
				t.Errorf("el comentario '#' se perdió o cortó el bloque:\n%s", codeUnit.Content)
			}
			if !strings.Contains(codeUnit.Content, "return resultado") {
				t.Errorf("el bloque de código quedó incompleto:\n%s", codeUnit.Content)
			}

			// Ningún concepto debe tener un título tomado de "Más texto
			// después del bloque de código." (fallback de título basura).
			for _, u := range units {
				if strings.HasPrefix(u.Title, "Más texto") {
					t.Errorf("se generó un título basura desde texto de párrafo: %q", u.Title)
				}
			}
		})
	}
}

func TestParseMarkdown_MultipleCodeBlocksNoneUnbalanced(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 5; i++ {
		sb.WriteString("## Sección ")
		sb.WriteString(string(rune('0' + i)))
		sb.WriteString("\n\nTexto.\n\n```python\n# comentario ")
		sb.WriteString(string(rune('0' + i)))
		sb.WriteString("\nx = 1\n```\n\n")
	}

	units := ParseMarkdown(sb.String())
	for _, u := range units {
		if n := strings.Count(u.Content, "```"); n%2 != 0 {
			t.Errorf("concepto %q tiene un número impar de líneas ``` (%d): backticks desbalanceados:\n%s", u.Title, n, u.Content)
		}
	}
}

// Sin ningún encabezado que sirva de límite de recuperación en el resto del
// documento, el bloque se extiende hasta el final del archivo — igual que
// antes de este fix, pero ahora además queda cerrado sintéticamente y la
// advertencia lo indica.
func TestParseMarkdown_UnclosedFenceWithNoRecoveryBoundaryLogsWarning(t *testing.T) {
	doc := "## Sección\n\nTexto.\n\n```python\nx = 1\nresultado sin cerrar el bloque nunca\n"

	units := ParseMarkdown(doc)

	if !anyNoteContains(units, "sin cerrar") {
		t.Fatalf("se esperaba una advertencia por bloque sin cerrar, unidades: %+v", units)
	}
	last := units[len(units)-1]
	if !strings.Contains(last.Content, "resultado sin cerrar el bloque nunca") {
		t.Errorf("el contenido final del bloque no llegó hasta el final del archivo:\n%s", last.Content)
	}
	if n := strings.Count(last.Content, "```"); n%2 != 0 {
		t.Errorf("el cierre sintético dejó backticks desbalanceados (%d): \n%s", n, last.Content)
	}
}

func anyNoteContains(units []Unit, substr string) bool {
	for _, u := range units {
		for _, note := range u.Notes {
			if strings.Contains(note, substr) {
				return true
			}
		}
	}
	return false
}

func TestParseMarkdown_PlainDocumentUnaffected(t *testing.T) {
	doc := "## Uno\n\nTexto uno.\n\n## Dos\n\nTexto dos.\n"
	units := ParseMarkdown(doc)
	if len(units) != 2 {
		t.Fatalf("se esperaban 2 unidades, hubo %d", len(units))
	}
	if units[0].Title != "Uno" || units[1].Title != "Dos" {
		t.Errorf("títulos = %v, se esperaba ['Uno', 'Dos']", titles(units))
	}
	if len(units[0].Notes) != 0 || len(units[1].Notes) != 0 {
		t.Errorf("no debería haber notas sin bloques de código sin cerrar")
	}
}

// BUG 1: un bloque de código fenced sin cierre real ya no absorbe el resto
// del documento — se recupera en el primer encabezado de nivel igual o
// superior al de la sección donde abrió el bloque.
func TestParseMarkdown_UnclosedFenceRecoversAtNextHeading(t *testing.T) {
	// Reproduce textualmente el [CASO 20] del documento adversarial: el
	// primer encabezado de nivel <= 2 que aparece tras el fence sin cerrar
	// es, literalmente, "Este encabezado queda atrapado..." — la
	// recuperación debe detenerse AHÍ (el primero que encuentra), no saltar
	// a una sección posterior más "conveniente".
	doc := "## [CASO 20] Bloque sin cerrar\n\n" +
		"```python\n" +
		"def sin_cerrar():\n" +
		"    return \"el bloque no tiene cierre\"\n\n" +
		"## Este encabezado queda atrapado dentro del bloque sin cerrar\n\n\n" +
		"## Sección de relleno 1\n\nTexto de relleno 1.\n\n" +
		"## Sección de relleno 2\n\nTexto de relleno 2.\n\n" +
		"## Cierre\n\nTexto de cierre.\n"

	units := ParseMarkdown(doc)

	wantTitles := []string{
		"[CASO 20] Bloque sin cerrar",
		"Este encabezado queda atrapado dentro del bloque sin cerrar",
		"Sección de relleno 1",
		"Sección de relleno 2",
		"Cierre",
	}
	if len(units) != len(wantTitles) {
		t.Fatalf("se esperaban %d unidades, hubo %d: %v", len(wantTitles), len(units), titles(units))
	}
	for i, want := range wantTitles {
		if units[i].Title != want {
			t.Errorf("unidad %d: título = %q, se esperaba %q", i, units[i].Title, want)
		}
	}

	casoUnit := units[0]
	if n := strings.Count(casoUnit.Content, "```"); n != 2 {
		t.Errorf("se esperaban 2 líneas ``` (apertura + cierre sintético), hubo %d:\n%s", n, casoUnit.Content)
	}
	if !strings.Contains(casoUnit.Content, "el bloque no tiene cierre") {
		t.Errorf("el contenido del bloque se perdió:\n%s", casoUnit.Content)
	}
	if strings.Contains(casoUnit.Content, "Este encabezado queda atrapado") {
		t.Errorf("el encabezado de recuperación no debería quedar dentro del bloque de código:\n%s", casoUnit.Content)
	}

	if !anyNoteContains(units, "cerca de la línea") || !anyNoteContains(units, "cierre implícito") {
		t.Errorf("falta la advertencia de recuperación con la línea de apertura y de cierre implícito: %+v", units)
	}

	// Las secciones DESPUÉS del punto de recuperación deben tener su
	// contenido normal (no quedaron atrapadas ni truncadas).
	for _, title := range []string{"Sección de relleno 1", "Sección de relleno 2", "Cierre"} {
		for _, u := range units {
			if u.Title == title && strings.TrimSpace(u.Content) == "" {
				t.Errorf("unidad %q quedó vacía tras la recuperación", title)
			}
		}
	}
}

// BUG 2a: bloques delimitados con tildes (~~~) se preservan completos.
func TestParseMarkdown_TildeFenceBlockStaysIntact(t *testing.T) {
	doc := "## [CASO 3] Bloque con tildes\n\n" +
		"~~~bash\n" +
		"#!/bin/bash\n" +
		"# comentario que parece encabezado\n" +
		"echo \"## tampoco esto\"\n" +
		"grep -c '```' archivo.md\n" +
		"~~~\n\n" +
		"Fin del caso 3.\n"

	units := ParseMarkdown(doc)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad, hubo %d: %v", len(units), titles(units))
	}
	c := units[0].Content

	for _, want := range []string{"#!/bin/bash", "# comentario que parece encabezado", "echo \"## tampoco esto\"", "grep -c '```' archivo.md", "Fin del caso 3."} {
		if !strings.Contains(c, want) {
			t.Errorf("falta %q en el contenido:\n%s", want, c)
		}
	}
	if n := strings.Count(c, "~~~"); n != 2 {
		t.Errorf("se esperaban 2 líneas ~~~ (apertura+cierre), hubo %d:\n%s", n, c)
	}
}

// BUG 2c: un ``` dentro de un bloque abierto con ~~~ es contenido literal,
// nunca un cierre — la simetría de delimitadores debe respetarse.
func TestParseMarkdown_BacktickInsideTildeFenceIsLiteral(t *testing.T) {
	doc := "## Caso\n\n~~~text\nlínea 1\n```\nlínea 2\n~~~\n\nDespués.\n"
	units := ParseMarkdown(doc)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad, hubo %d", len(units))
	}
	c := units[0].Content
	if !strings.Contains(c, "línea 1") || !strings.Contains(c, "```") || !strings.Contains(c, "línea 2") {
		t.Errorf("el contenido del bloque de tildes se perdió o se cortó prematuramente:\n%s", c)
	}
	if !strings.Contains(c, "Después.") {
		t.Errorf("el texto posterior al bloque no quedó en el mismo concepto:\n%s", c)
	}
}

// BUG 2b: bloques indentados con 4 espacios también se detectan en la ruta
// de Markdown (antes solo funcionaba en texto plano).
func TestParseMarkdown_IndentedCodeBlockStaysIntact(t *testing.T) {
	doc := "## [CASO 4] Bloque indentado\n\n" +
		"    def funcion_indentada(x):\n" +
		"        # Comentario con almohadilla\n" +
		"        return x * 2\n\n" +
		"    ## Segunda linea que parece encabezado\n\n" +
		"Texto normal después del bloque indentado.\n"

	units := ParseMarkdown(doc)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad, hubo %d: %v", len(units), titles(units))
	}
	c := units[0].Content

	if !strings.Contains(c, "def funcion_indentada(x):") {
		t.Errorf("falta la línea de definición de función:\n%s", c)
	}
	if !strings.Contains(c, "# Comentario con almohadilla") {
		t.Errorf("falta el comentario dentro del bloque:\n%s", c)
	}
	if !strings.Contains(c, "## Segunda linea que parece encabezado") {
		t.Errorf("la 'segunda línea' indentada debería quedar como código, no perderse:\n%s", c)
	}
	if !strings.Contains(c, "Texto normal después del bloque indentado.") {
		t.Errorf("el texto posterior al bloque no quedó en el mismo concepto:\n%s", c)
	}

	for _, u := range units {
		if strings.HasPrefix(u.Title, "def ") || strings.HasPrefix(u.Title, "#!/") || strings.Contains(u.Title, "Comentario") || strings.Contains(u.Title, "Segunda linea") {
			t.Errorf("título tomado de una línea de código/comentario en vez de un encabezado real: %q", u.Title)
		}
	}
}

// Regresión: un bloque de 4 backticks con uno de 3 anidado adentro (CASO 2)
// debe seguir intacto — la longitud del delimitador de apertura importa
// (matchFenceClose exige longitud >= la de apertura), no solo el carácter.
func TestParseMarkdown_NestedShorterFenceInsideLongerFenceIsLiteral(t *testing.T) {
	doc := "## [CASO 2] Anidado\n\n" +
		"````markdown\n" +
		"Ejemplo:\n\n" +
		"```go\n" +
		"func main() {\n" +
		"    // # no es encabezado\n" +
		"}\n" +
		"```\n" +
		"````\n\n" +
		"Fin del caso 2.\n"

	units := ParseMarkdown(doc)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad, hubo %d: %v", len(units), titles(units))
	}
	c := units[0].Content
	if !strings.Contains(c, "```go") || !strings.Contains(c, "func main()") || !strings.Contains(c, "Fin del caso 2.") {
		t.Errorf("el bloque anidado se partió incorrectamente:\n%s", c)
	}
}

func titles(units []Unit) []string {
	out := make([]string, len(units))
	for i, u := range units {
		out[i] = u.Title
	}
	return out
}
