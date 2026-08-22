package converter

import (
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
