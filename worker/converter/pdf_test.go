package converter

import (
	"strings"
	"testing"
)

// unitsFromPages se prueba de forma aislada del binario pdftotext: opera
// sobre páginas ya extraídas, así que no depende de tener poppler instalado.

func TestUnitsFromPages_GroupsMultiPageSectionsUnderOneHeading(t *testing.T) {
	pages := []string{
		"1. Contexto\nPárrafo inicial de la sección uno.",
		"Continúa la sección uno en la siguiente diapositiva.",
		"2. Objetivos\nPárrafo de la sección dos.",
	}

	units := unitsFromPages(pages)

	if len(units) != 2 {
		t.Fatalf("se esperaban 2 unidades (una por sección numerada), hubo %d", len(units))
	}
	if units[0].Title != "1. Contexto" {
		t.Errorf("título de la primera unidad = %q, se esperaba '1. Contexto'", units[0].Title)
	}
	if !strings.Contains(units[0].Content, "Continúa la sección uno") {
		t.Errorf("la segunda página de la sección uno no se fusionó en el contenido: %q", units[0].Content)
	}
	if units[1].Title != "2. Objetivos" {
		t.Errorf("título de la segunda unidad = %q, se esperaba '2. Objetivos'", units[1].Title)
	}
}

func TestUnitsFromPages_TitleNotDuplicatedInBody(t *testing.T) {
	pages := []string{
		"1. Contexto\nPárrafo inicial de la sección uno.",
		"2. Objetivos\nPárrafo de la sección dos.",
	}

	units := unitsFromPages(pages)

	for _, u := range units {
		if strings.Contains(u.Content, u.Title) {
			t.Errorf("el título %q aparece duplicado dentro del cuerpo: %q", u.Title, u.Content)
		}
	}
}

func TestUnitsFromPages_MergesCoverPageIntoFirstSection(t *testing.T) {
	pages := []string{
		"Portada del documento",
		"1. Contexto\nPárrafo inicial.",
	}

	units := unitsFromPages(pages)

	if len(units) != 1 {
		t.Fatalf("se esperaba que la portada se fusionara con la sección 1, hubo %d unidades", len(units))
	}
	if units[0].Title != "1. Contexto" {
		t.Errorf("título = %q, se esperaba que el encabezado numerado siguiera siendo el título", units[0].Title)
	}
	if !strings.Contains(units[0].Content, "Portada del documento") {
		t.Errorf("el contenido de la portada se perdió: %q", units[0].Content)
	}
}

func TestUnitsFromPages_MergesShortTrailingPageIntoLastSection(t *testing.T) {
	pages := []string{
		"1. Contexto\nPárrafo inicial de la sección uno con suficiente longitud.",
		"www-ejemplo-com",
	}

	units := unitsFromPages(pages)

	if len(units) != 1 {
		t.Fatalf("se esperaba que la contraportada corta se fusionara con la sección anterior, hubo %d unidades", len(units))
	}
	if !strings.Contains(units[0].Content, "www-ejemplo-com") {
		t.Errorf("el contenido de la contraportada se perdió: %q", units[0].Content)
	}
}

func TestUnitsFromPages_RelabelsCoverThatRepeatsFirstSectionTitle(t *testing.T) {
	pages := []string{
		"1. Contexto\nLecturas para Arquitectos (No. 2)\nIntroducción biográfica del autor.\nISBN: 978-0-000000-0",
		"1. Contexto\nContenido real de la sección uno.",
		"2. Objetivos\nContenido de la sección dos.",
	}

	units := unitsFromPages(pages)

	if len(units) != 3 {
		t.Fatalf("se esperaban 3 unidades, hubo %d", len(units))
	}
	if units[0].Title != "Preliminares" {
		t.Errorf("título de la primera unidad = %q, se esperaba 'Preliminares'", units[0].Title)
	}
	if !strings.Contains(units[0].Content, "ISBN") {
		t.Errorf("el contenido de la portada se perdió al reetiquetarla: %q", units[0].Content)
	}
	if units[1].Title != "1. Contexto" {
		t.Errorf("título de la segunda unidad = %q, se esperaba que conservara '1. Contexto'", units[1].Title)
	}
	if !strings.Contains(units[1].Content, "Contenido real de la sección uno") {
		t.Errorf("el contenido real de la sección uno no quedó en la unidad correcta: %q", units[1].Content)
	}
}

func TestUnitsFromPages_FallsBackToPerPageWithoutNumberedHeadings(t *testing.T) {
	pages := []string{
		"Primera diapositiva sin encabezado numerado, con contenido suficientemente largo para no fusionarse con la siguiente página del documento.",
		"Segunda diapositiva sin encabezado numerado, también con contenido suficientemente largo para no fusionarse con la anterior.",
	}

	units := unitsFromPages(pages)

	if len(units) != 2 {
		t.Fatalf("sin encabezados numerados se esperaba una unidad por página, hubo %d", len(units))
	}
}
