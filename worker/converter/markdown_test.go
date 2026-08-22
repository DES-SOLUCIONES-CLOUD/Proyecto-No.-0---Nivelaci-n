package converter

import (
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
