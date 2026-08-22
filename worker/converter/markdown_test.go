package converter

import (
	"strings"
	"testing"
)

func TestMakeSlug(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		expectedSlug string
		expectedFallback bool
	}{
		{
			name:         "Título normal",
			title:        "Este es un título normal",
			expectedSlug: "concepto-01-este-es-un-titulo-normal.md",
			expectedFallback: false,
		},
		{
			name:         "Con símbolos",
			title:        "1. ¿Qué es Design Thinking?",
			expectedSlug: "concepto-02-1-que-es-design-thinking.md",
			expectedFallback: false,
		},
		{
			name:         "Mojibake",
			title:        "'HVLJQ7KLQNLQJ:RUNVKRS",
			expectedSlug: "concepto-03-hvljq7klqnlqj-runvkrs.md",
			expectedFallback: false, // Tiene > 3 letras, así que no usará fallback, pero los símbolos se limpian. 
		},
		{
			name:         "Solo símbolos (fallback)",
			title:        "!@#$%^&*",
			expectedSlug: "concepto-04-concepto.md",
			expectedFallback: true,
		},
		{
			name:         "Pocas letras (fallback)",
			title:        "A B",
			expectedSlug: "concepto-05-concepto.md",
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

func TestParseMarkdown_UnclosedFenceLogsWarningInsteadOfSilentFailure(t *testing.T) {
	doc := "## Sección\n\nTexto.\n\n```python\nx = 1\n# sin cerrar el bloque\n"

	units := ParseMarkdown(doc)
	last := units[len(units)-1]

	found := false
	for _, note := range last.Notes {
		if strings.Contains(note, "sin cerrar") {
			found = true
		}
	}
	if !found {
		t.Errorf("se esperaba una nota de advertencia por bloque sin cerrar, notas: %v", last.Notes)
	}
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

func titles(units []Unit) []string {
	out := make([]string, len(units))
	for i, u := range units {
		out[i] = u.Title
	}
	return out
}
