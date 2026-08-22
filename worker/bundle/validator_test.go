package bundle

import (
	"testing"
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
			if got := isSuspiciousText(tt.text); got != tt.expected {
				t.Errorf("isSuspiciousText() = %v, se esperaba %v para %q", got, tt.expected, tt.text)
			}
		})
	}
}
