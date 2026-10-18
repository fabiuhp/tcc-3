package transcripts

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 2 << 20

// ReadMarkdown preserves the source text, including speakers, timestamps and formatting.
func ReadMarkdown(name string, reader io.Reader) (string, error) {
	if !strings.EqualFold(filepath.Ext(name), ".md") {
		return "", fmt.Errorf("a transcricao deve ser um arquivo .md")
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("nao foi possivel ler a transcricao: %w", err)
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	if len(data) > MaxBytes {
		return "", fmt.Errorf("a transcricao deve ter no maximo 2 MiB")
	}
	if err := Validate(text); err != nil {
		return "", err
	}
	return text, nil
}

func Validate(text string) error {
	if len(text) > MaxBytes {
		return fmt.Errorf("a transcricao deve ter no maximo 2 MiB")
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("a transcricao esta vazia; envie um .md com o texto da reuniao")
	}
	if !utf8.ValidString(text) || strings.ContainsRune(text, '\x00') {
		return fmt.Errorf("a transcricao deve conter texto UTF-8 valido, nao um arquivo binario")
	}
	return nil
}
