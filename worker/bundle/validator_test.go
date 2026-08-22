package bundle

import (
	"strings"
	"testing"

	"okf-worker/converter"
)

func TestIsSuspiciousText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "Texto normal válido",
			text:     "Este es un texto completamente normal que no debería ser marcado como sospechoso porque tiene una cantidad razonable de vocales.",
			expected: false,
		},
		{
			name:     "Texto demasiado corto",
			text:     "Hola mundo",
			expected: false, // Menos de 50 caracteres no se analiza
		},
		{
			name:     "Mojibake / sin vocales",
			text:     "'HVLJQ7KLQNLQJ:RUNVKRS xyz prst qwrt xyz prst qwrt xyz prst qwrt",
			expected: true,
		},
		{
			name:     "Demasiados símbolos",
			text:     "texto con @#%& @#%& @#%& @#%& @#%& @#%& @#%& @#%& @#%& @#%& @#%& @#%& @#%& @#%&",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := isSuspiciousText(tt.text)
			if got != tt.expected {
				t.Errorf("isSuspiciousText() = %v, se esperaba %v para %q", got, tt.expected, tt.text)
			}
			if got && reason == "" {
				t.Errorf("isSuspiciousText() marcó sospechoso pero no dio razón para %q", tt.text)
			}
			if !got && reason != "" {
				t.Errorf("isSuspiciousText() no marcó sospechoso pero devolvió razón %q", reason)
			}
		})
	}
}

// Caso real reportado: concepto-23-tabla-de-datos-12.md — una tabla Markdown
// perfectamente válida (identificadores con guiones tipo "labore-12-0-0")
// que antes disparaba "texto sospechoso" por su alta densidad natural de
// guiones y dígitos frente al resto del archivo.
func TestDetectWarnings_ValidMarkdownTableIsNotSuspicious(t *testing.T) {
	content := "# Tabla de datos 12\n\n" +
		"| Columna 1 | Columna 2 | Columna 3 | Columna 4 |\n" +
		"| --- | --- | --- | --- |\n" +
		"| labore-12-0-0 | excepteur-12-0-1 | esse-12-0-2 | laborum-12-0-3 |\n" +
		"| aliquip-12-1-0 | ea-12-1-1 | culpa-12-1-2 | nostrud-12-1-3 |\n" +
		"| amet-12-2-0 | ex-12-2-1 | mollit-12-2-2 | eu-12-2-3 |\n" +
		"| aliqua-12-3-0 | occaecat-12-3-1 | dolor-12-3-2 | voluptate-12-3-3 |\n" +
		"| velit-12-4-0 | esse-12-4-1 | incididunt-12-4-2 | amet-12-4-3 |\n" +
		"| reprehenderit-12-5-0 | do-12-5-1 | minim-12-5-2 | dolore-12-5-3 |\n\n" +
		"---\n\n[← Volver al índice](index.md)"

	units := []converter.Unit{{Title: "Tabla de datos 12", Content: content, Slug: "concepto-23-tabla-de-datos-12.md"}}
	warnings := detectWarnings(units)
	for _, w := range warnings {
		if strings.Contains(w, "Texto sospechoso") {
			t.Errorf("una tabla Markdown válida no debería disparar 'texto sospechoso': %s", w)
		}
	}
}

// Una tabla no debe "blindar" al resto del concepto: si además de la tabla
// hay prosa realmente corrupta (p. ej. columnas de PDF mezcladas a nivel de
// caracteres), la advertencia debe seguir disparando sobre esa prosa.
func TestDetectWarnings_CorruptedProseNextToValidTableStillFlagged(t *testing.T) {
	corrupted := strings.Repeat("'HVLJQ7KLQNLQJ:RUNVKRS xyz prst qwrt xyz prst qwrt xyz prst qwrt bcdfg hjklmn. ", 5)
	content := "# Sección\n\n" +
		corrupted + "\n\n" +
		"| Columna 1 | Columna 2 |\n" +
		"| --- | --- |\n" +
		"| dato-0-0 | dato-0-1 |\n\n" +
		"---\n\n[← Volver al índice](index.md)"

	units := []converter.Unit{{Title: "Sección", Content: content, Slug: "concepto-01-seccion.md"}}
	warnings := detectWarnings(units)

	found := false
	for _, w := range warnings {
		if strings.Contains(w, "Texto sospechoso") {
			found = true
			if !strings.Contains(w, "razón:") {
				t.Errorf("la advertencia debería incluir la razón específica: %q", w)
			}
		}
	}
	if !found {
		t.Errorf("prosa corrupta junto a una tabla válida debería seguir disparando 'texto sospechoso', advertencias: %v", warnings)
	}
}

// El encoding roto (mojibake) en un párrafo de prosa debe seguir detectándose.
func TestDetectWarnings_MojibakeProseStillFlagged(t *testing.T) {
	corrupted := strings.Repeat("Xzqvk bcdfgh jklmnp qrstvw xzbcdf ghjklm npqrst vwxzbc dfghjk lmnpqr. ", 5)
	content := "# Sección\n\n" +
		corrupted + "\n\n" +
		"---\n\n[← Volver al índice](index.md)"

	units := []converter.Unit{{Title: "Sección", Content: content, Slug: "concepto-02-seccion.md"}}
	warnings := detectWarnings(units)

	found := false
	for _, w := range warnings {
		if strings.Contains(w, "Texto sospechoso") {
			found = true
		}
	}
	if !found {
		t.Errorf("encoding roto (mojibake) debería seguir disparando 'texto sospechoso', advertencias: %v", warnings)
	}
}

func TestExtractProseText_StripsTablesListsAndCodeBlocks(t *testing.T) {
	content := "Prosa antes.\n\n" +
		"| A | B |\n| --- | --- |\n| 1 | 2 |\n\n" +
		"- ítem 1\n- ítem 2\n\n" +
		"```python\n# comentario\nx = 1\n```\n\n" +
		"Prosa después."

	prose := extractProseText(content)

	if strings.Contains(prose, "| A | B |") || strings.Contains(prose, "| 1 | 2 |") {
		t.Errorf("la tabla no se removió de la prosa: %q", prose)
	}
	if strings.Contains(prose, "ítem 1") {
		t.Errorf("la lista no se removió de la prosa: %q", prose)
	}
	if strings.Contains(prose, "x = 1") || strings.Contains(prose, "# comentario") {
		t.Errorf("el bloque de código no se removió de la prosa: %q", prose)
	}
	if !strings.Contains(prose, "Prosa antes.") || !strings.Contains(prose, "Prosa después.") {
		t.Errorf("la prosa real se perdió: %q", prose)
	}
}
