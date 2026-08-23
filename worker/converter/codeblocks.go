package converter

import (
	"regexp"
	"strings"
)

// Detección de bloques de código compartida entre el segmentador de Markdown
// (markdown.go) y el de texto plano (plaintext.go). Antes cada ruta traía su
// propia copia de esta lógica, y un fix (código indentado) se aplicó solo a
// texto plano — este archivo es el punto único de verdad para que eso no
// vuelva a pasar.

// fenceOpenRegex reconoce la línea de APERTURA de un bloque de código
// fenced: una racha de 3 o más backticks o 3 o más tildes, seguida
// opcionalmente de un identificador de lenguaje sin espacios
// ("```python", "~~~bash", "````markdown", "```", "~~~").
var fenceOpenRegex = regexp.MustCompile("^(`{3,}|~{3,})(\\S*)$")

// matchFenceOpen determina si trimmed abre un bloque fenced. Retorna el
// carácter delimitador ('`' o '~') y su longitud (>= 3), o (0, 0) si la
// línea no abre un bloque.
func matchFenceOpen(trimmed string) (rune, int) {
	m := fenceOpenRegex.FindStringSubmatch(trimmed)
	if m == nil {
		return 0, 0
	}
	run := m[1]
	return rune(run[0]), len(run)
}

// matchFenceClose determina si trimmed cierra un bloque abierto con el
// carácter ch y longitud mínima minLen: debe ser una racha del MISMO
// carácter, de longitud igual o mayor, y nada más en la línea. Por eso un
// ``` que aparece dentro de un bloque abierto con ~~~ (o una racha de
// backticks más corta que la de apertura, como en un bloque de 4 backticks
// que contiene uno de 3 anidado) nunca cuenta como cierre: es contenido
// literal del bloque, tal como exige CommonMark.
func matchFenceClose(trimmed string, ch rune, minLen int) bool {
	if len(trimmed) < minLen {
		return false
	}
	for _, r := range trimmed {
		if r != ch {
			return false
		}
	}
	return true
}

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
