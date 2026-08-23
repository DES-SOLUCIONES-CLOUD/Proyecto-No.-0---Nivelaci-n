package handlers

import "testing"

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"documento.md", "markdown"},
		{"documento.markdown", "markdown"},
		{"pagina.html", "html"},
		{"pagina.htm", "html"},
		{"notas.txt", "plaintext"},
		{"reporte.pdf", "pdf"},
		{"informe.docx", "docx"},
		{"viejo.doc", "doc_legacy"},
		{"MAYUSCULAS.PDF", "pdf"},
		{"sin_extension", "unknown"},
		{"imagen.png", "unknown"},
		{"archivo.tar.gz", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := detectFormat(tt.filename); got != tt.want {
				t.Errorf("detectFormat(%q) = %q, se esperaba %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"documento normal.pdf", "documento_normal.pdf"},
		{"../../etc/passwd", "passwd"},
		{"carpeta/archivo.txt", "archivo.txt"},
		{"a..b.txt", "a_b.txt"},
		{"..archivo.txt", "_archivo.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.name)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, se esperaba %q", tt.name, got, tt.want)
			}
		})
	}
}

// sanitizeFilename es la única barrera contra path traversal en el nombre
// original del archivo subido: nunca debe dejar pasar un "/" o "\\" que
// permita escribir fuera de la carpeta del job en MinIO.
func TestSanitizeFilename_NeverLeavesPathSeparators(t *testing.T) {
	inputs := []string{
		"../../../etc/passwd",
		"..\\..\\secrets.txt",
		"a/b/c/d.txt",
		"////etc/passwd",
	}
	for _, in := range inputs {
		got := sanitizeFilename(in)
		if got == "" {
			t.Errorf("sanitizeFilename(%q) devolvió cadena vacía", in)
			continue
		}
		for _, ch := range got {
			if ch == '/' || ch == '\\' {
				t.Errorf("sanitizeFilename(%q) = %q todavía contiene un separador de ruta", in, got)
			}
		}
	}
}

func TestContentTypeFor(t *testing.T) {
	tests := []struct {
		format string
		want   string
	}{
		{"markdown", "text/markdown"},
		{"html", "text/html"},
		{"pdf", "application/pdf"},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"plaintext", "text/plain"},
		{"unknown", "text/plain"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			if got := contentTypeFor(tt.format); got != tt.want {
				t.Errorf("contentTypeFor(%q) = %q, se esperaba %q", tt.format, got, tt.want)
			}
		})
	}
}
