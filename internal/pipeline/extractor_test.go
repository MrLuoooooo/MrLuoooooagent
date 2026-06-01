package pipeline

import (
	"strings"
	"testing"
)

func TestExtractText_TXT(t *testing.T) {
	content := "Hello, this is a plain text file.\nWith multiple lines."
	result, err := ExtractText([]byte(content), "test.txt")
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected 'Hello' in result, got: %s", result)
	}
}

func TestExtractText_MD(t *testing.T) {
	content := "# Title\n\nThis is **markdown** content."
	result, err := ExtractText([]byte(content), "readme.md")
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(result, "markdown") {
		t.Errorf("expected markdown content, got: %s", result)
	}
}

func TestExtractText_UnknownExtension(t *testing.T) {
	result, err := ExtractText([]byte("some data"), "file.xyz")
	// Should return an error for unsupported formats
	if err == nil {
		t.Error("expected error for unsupported extension")
	}
	if result != "some data" {
		t.Errorf("expected raw data fallback, got: %s", result)
	}
}

func TestExtractText_NoExtension(t *testing.T) {
	content := "data without extension"
	result, err := ExtractText([]byte(content), "Makefile")
	if err == nil {
		t.Error("expected error for no extension")
	}
	if result != content {
		t.Errorf("expected raw content, got: %s", result)
	}
}

func TestExtractText_Empty(t *testing.T) {
	result, err := ExtractText([]byte{}, "empty.txt")
	if err != nil {
		t.Fatalf("ExtractText empty: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got: %s", result)
	}
}

func TestExtractText_CodeExtensions(t *testing.T) {
	// Makefile with no extension is unsupported
	result, err := ExtractText([]byte("int main() {}"), "main.go")
	if err != nil {
		t.Fatalf("ExtractText .go: %v", err)
	}
	if !strings.Contains(result, "main") {
		t.Errorf("expected go content, got: %s", result)
	}
}

func TestExtractText_PDFInvalidData(t *testing.T) {
	// Random bytes that are not a valid PDF
	_, err := ExtractText([]byte("not a pdf file indeed random bytes"), "doc.pdf")
	if err == nil {
		t.Error("expected error for invalid PDF data")
	}
}

func TestExtractText_DOCXInvalidData(t *testing.T) {
	// Random bytes that are not a valid DOCX
	_, err := ExtractText([]byte("not a valid docx"), "doc.docx")
	if err == nil {
		t.Error("expected error for invalid DOCX data")
	}
}

func TestExtractText_XLSXInvalidData(t *testing.T) {
	_, err := ExtractText([]byte("not xlsx"), "data.xlsx")
	if err == nil {
		t.Error("expected error for invalid XLSX data")
	}
}

func TestExtractText_PPTXInvalidData(t *testing.T) {
	_, err := ExtractText([]byte("not pptx"), "slides.pptx")
	if err == nil {
		t.Error("expected error for invalid PPTX data")
	}
}

func TestExtractText_CSV(t *testing.T) {
	content := "name,age\nAlice,30\nBob,25"
	result, err := ExtractText([]byte(content), "data.csv")
	if err != nil {
		t.Fatalf("ExtractText csv: %v", err)
	}
	if !strings.Contains(result, "Alice") {
		t.Errorf("expected CSV content, got: %s", result)
	}
}

func TestExtractText_JSON(t *testing.T) {
	content := `{"name": "test", "value": 42}`
	result, err := ExtractText([]byte(content), "config.json")
	if err != nil {
		t.Fatalf("ExtractText json: %v", err)
	}
	if !strings.Contains(result, "test") {
		t.Errorf("expected JSON content, got: %s", result)
	}
}

// --- Unit tests for inner extraction functions ---

func TestExtractXMLText_Simple(t *testing.T) {
	xml := `<root><item>hello</item><item>world</item></root>`
	result := extractXMLText(xml)
	if !strings.Contains(result, "hello") || !strings.Contains(result, "world") {
		t.Errorf("extractXMLText = %q", result)
	}
}

func TestExtractXMLText_Empty(t *testing.T) {
	result := extractXMLText("<root></root>")
	if result != "" {
		t.Errorf("expected empty, got: %q", result)
	}
}

func TestExtractXMLText_Nested(t *testing.T) {
	xml := `<doc><body><p>Hello <b>World</b></p></body></doc>`
	result := extractXMLText(xml)
	if !strings.Contains(result, "Hello") || !strings.Contains(result, "World") {
		t.Errorf("extractXMLText = %q", result)
	}
}

func TestParseXLSXSharedStrings_Empty(t *testing.T) {
	result := parseXLSXSharedStrings("<sst></sst>")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseXLSXSharedStrings_InvalidXML(t *testing.T) {
	result := parseXLSXSharedStrings("not xml")
	if len(result) != 0 {
		t.Errorf("expected empty map for invalid XML, got %d", len(result))
	}
}

func TestParseXLSXSheet_Empty(t *testing.T) {
	result := parseXLSXSheet("<worksheet></worksheet>", nil)
	if result != "" {
		t.Errorf("expected empty, got: %q", result)
	}
}

func TestExtractPPTXSlideText_Empty(t *testing.T) {
	result := extractPPTXSlideText("<sld></sld>")
	if result != "" {
		t.Errorf("expected empty for empty slide, got: %q", result)
	}
}

func TestExtractPPTXSlideText_InvalidXML(t *testing.T) {
	// Invalid XML will fall back to generic extraction
	result := extractPPTXSlideText("not xml>text<stuff")
	if result != "text" {
		t.Errorf("expected fallback extraction, got: %q", result)
	}
}
