package converter

import (
	"strings"
	"testing"
)

func TestParsePlainText_SingleSection(t *testing.T) {
	content := "Este es un documento de texto plano.\nNo tiene encabezados.\nSolo texto normal."

	units := ParsePlainText(content)

	if len(units) != 1 {
		t.Fatalf("Se esperaba 1 unidad, se obtuvieron %d", len(units))
	}

	if units[0].Title != "Documento" {
		t.Errorf("Se esperaba título 'Documento', se obtuvo '%s'", units[0].Title)
	}
}

func TestParsePlainText_MultipleHeaders(t *testing.T) {
	content := `
# Título Principal
Este es el contenido del primer título.
## Subtítulo 1
Contenido del subtítulo 1.
### Subtítulo 2
Contenido del subtítulo 2.
`

	units := ParsePlainText(content)

	if len(units) != 3 {
		t.Fatalf("Se esperaban 3 unidades, se obtuvieron %d", len(units))
	}

	if units[0].Title != "Título Principal" {
		t.Errorf("Unidad 1: se esperaba 'Título Principal', se obtuvo '%s'", units[0].Title)
	}
	if units[1].Title != "Subtítulo 1" {
		t.Errorf("Unidad 2: se esperaba 'Subtítulo 1', se obtuvo '%s'", units[1].Title)
	}
	if units[2].Title != "Subtítulo 2" {
		t.Errorf("Unidad 3: se esperaba 'Subtítulo 2', se obtuvo '%s'", units[2].Title)
	}
}

func TestParsePlainText_SetextHeadingsBothLevels(t *testing.T) {
	content := "Título Documento\n=================\n\n" +
		"Texto del documento.\n\n" +
		"Sección 1: Algo\n---------------\n\n" +
		"Texto de la sección uno.\n"

	units := ParsePlainText(content)
	if len(units) != 2 {
		t.Fatalf("se esperaban 2 unidades, hubo %d: %v", len(units), titlesOf(units))
	}
	if units[0].Title != "Título Documento" {
		t.Errorf("título nivel 1 = %q, se esperaba 'Título Documento'", units[0].Title)
	}
	if units[1].Title != "Sección 1: Algo" {
		t.Errorf("título nivel 2 = %q, se esperaba 'Sección 1: Algo'", units[1].Title)
	}
	if strings.Contains(units[0].Content, "===") || strings.Contains(units[1].Content, "---------------") {
		t.Errorf("el subrayado no debería quedar en el contenido")
	}
}

func TestParsePlainText_SeparatorLineDoesNotBecomeHeading(t *testing.T) {
	content := "Sección 1: Algo\n----------------\n\n" +
		"Texto uno.\n\n" +
		"----------------------------------------\n\n" +
		"Sección 2: Otro\n----------------\n\n" +
		"Texto dos.\n"

	units := ParsePlainText(content)
	if len(units) != 2 {
		t.Fatalf("el separador de guiones no debería generar una unidad extra: %d unidades: %v", len(units), titlesOf(units))
	}
	if units[0].Title != "Sección 1: Algo" || units[1].Title != "Sección 2: Otro" {
		t.Errorf("títulos = %v", titlesOf(units))
	}
}

func TestParsePlainText_IndentedCodeBlockNotSplitByHashComment(t *testing.T) {
	content := "Ejemplo de código 7\n-------------------\n\n" +
		"    def funcion_7(x, y):\n" +
		"        # Comentario explicativo aquí\n" +
		"        resultado = x + y\n" +
		"        return resultado\n\n" +
		"Texto después del bloque de código.\n"

	units := ParsePlainText(content)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad (el bloque de código no debe partirse), hubo %d: %v", len(units), titlesOf(units))
	}
	u := units[0]
	if u.Title != "Ejemplo de código 7" {
		t.Errorf("título = %q, se esperaba 'Ejemplo de código 7' (nunca una línea de código ni un comentario)", u.Title)
	}
	if fences := strings.Count(u.Content, "```"); fences != 2 {
		t.Errorf("se esperaban 2 líneas ``` (apertura+cierre), hubo %d:\n%s", fences, u.Content)
	}
	if !strings.Contains(u.Content, "# Comentario explicativo aquí") {
		t.Errorf("el comentario '#' se perdió o cortó el bloque:\n%s", u.Content)
	}
	if !strings.Contains(u.Content, "return resultado") {
		t.Errorf("el bloque de código quedó incompleto:\n%s", u.Content)
	}
	if !strings.Contains(u.Content, "Texto después del bloque de código.") {
		t.Errorf("el texto posterior al bloque se perdió:\n%s", u.Content)
	}
}

func TestParsePlainText_NoUnitHasZeroContentForCodeExample(t *testing.T) {
	content := "Ejemplo de código 7\n-------------------\n\n" +
		"    def funcion_7(x, y):\n" +
		"        # Comentario explicativo aquí\n" +
		"        resultado = x + y\n" +
		"        return resultado\n\n" +
		"Sección 8: Duis do\n------------------\n\n" +
		"Texto de la sección ocho.\n"

	units := ParsePlainText(content)
	for _, u := range units {
		if strings.TrimSpace(u.Content) == "" {
			t.Errorf("unidad %q quedó con 0 caracteres de contenido", u.Title)
		}
	}
}

func TestParsePlainText_NoTitleFromProseFallback(t *testing.T) {
	// Antes del fix, un párrafo de prosa arbitrario (sin ningún encabezado
	// real) podía terminar como título de una unidad. Ahora, si no hay
	// encabezado real, se usa un título genérico explícito.
	content := "Sección 1: Algo\n----------------\n\n" +
		"Primer párrafo de prosa normal, sin ningún encabezado.\n\n" +
		"Segundo párrafo de prosa normal, tampoco es un encabezado.\n\n" +
		"Sección 2: Otro\n----------------\n\nTexto.\n"

	units := ParsePlainText(content)
	for _, u := range units {
		if strings.Contains(u.Title, "prosa normal") || strings.HasSuffix(u.Title, ".") {
			t.Errorf("el título parece haber sido tomado de una oración de prosa: %q", u.Title)
		}
	}
}

func TestParsePlainText_ASCIITableBecomesMarkdownTable(t *testing.T) {
	content := "Tabla de datos 3\n----------------\n\n" +
		"+---------------+---------------+\n" +
		"|   Columna 1   |   Columna 2   |\n" +
		"+---------------+---------------+\n" +
		"|    et-3-0-0   |   anim-3-0-1  |\n" +
		"| eiusmod-3-1-0 |   aute-3-1-1  |\n" +
		"+---------------+---------------+\n\n" +
		"Texto después de la tabla.\n"

	units := ParsePlainText(content)
	if len(units) != 1 {
		t.Fatalf("se esperaba 1 unidad, hubo %d", len(units))
	}
	content2 := units[0].Content

	if !strings.Contains(content2, "| Columna 1 | Columna 2 |") {
		t.Errorf("falta la fila de encabezado Markdown:\n%s", content2)
	}
	if !strings.Contains(content2, "| --- | --- |") {
		t.Errorf("falta la fila separadora Markdown:\n%s", content2)
	}
	if !strings.Contains(content2, "| et-3-0-0 | anim-3-0-1 |") {
		t.Errorf("falta la primera fila de datos:\n%s", content2)
	}
	if strings.Contains(content2, "+---") {
		t.Errorf("los bordes ASCII no deberían quedar en la salida:\n%s", content2)
	}
}

func TestParsePlainText_IndentedListsAndBlockquote(t *testing.T) {
	content := "Sección 1: Algo\n----------------\n\n" +
		"Texto.\n\n" +
		"  1. Primer paso\n" +
		"  2. Segundo paso\n\n" +
		"  - Ítem uno\n" +
		"  - Ítem dos\n\n" +
		"    > Una cita indentada.\n"

	units := ParsePlainText(content)
	c := units[0].Content

	if !strings.Contains(c, "1. Primer paso") || !strings.Contains(c, "2. Segundo paso") {
		t.Errorf("la lista numerada no se convirtió correctamente:\n%s", c)
	}
	if !strings.Contains(c, "- Ítem uno") || !strings.Contains(c, "- Ítem dos") {
		t.Errorf("la lista con viñetas no se convirtió correctamente:\n%s", c)
	}
	if !strings.Contains(c, "> Una cita indentada.") {
		t.Errorf("la cita indentada no se convirtió a blockquote:\n%s", c)
	}
}

func titlesOf(units []Unit) []string {
	out := make([]string, len(units))
	for i, u := range units {
		out[i] = u.Title
	}
	return out
}
