package transcripts

import (
	"strings"
	"testing"
)

func TestReadMarkdownPreservesMeetTranscript(t *testing.T) {
	text := "## **Reuniao - Transcrição**\r\n\r\n### **00:01:28**\r\n\r\n**Cliente:** Preciso acompanhar o pedido.\r\n"
	got, err := ReadMarkdown("Reunião.MD", strings.NewReader("\ufeff"+text))
	if err != nil || got != text {
		t.Fatalf("ReadMarkdown = %q, %v, want original text", got, err)
	}
}

func TestReadMarkdownRejectsInvalidFiles(t *testing.T) {
	for _, test := range []struct {
		name, filename, text, message string
	}{
		{"extension", "meeting.mp3", "text", ".md"},
		{"empty", "meeting.md", "", "vazia"},
		{"whitespace", "meeting.md", "\ufeff \n\t", "vazia"},
		{"binary", "meeting.md", "hello\x00world", "UTF-8"},
		{"encoding", "meeting.md", "\xff\xfe", "UTF-8"},
		{"too large", "meeting.md", strings.Repeat("a", MaxBytes+1), "2 MiB"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ReadMarkdown(test.filename, strings.NewReader(test.text))
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestReadMarkdownAcceptsSizeLimit(t *testing.T) {
	if _, err := ReadMarkdown("meeting.md", strings.NewReader(strings.Repeat("a", MaxBytes))); err != nil {
		t.Fatal(err)
	}
}
