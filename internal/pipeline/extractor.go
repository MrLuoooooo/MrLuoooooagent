package pipeline

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractText extracts plain text from file content based on file extension.
// Supported formats: .pdf, .docx, .xlsx, .pptx, .txt, .md
func ExtractText(data []byte, filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return extractPDF(data)
	case ".docx":
		return extractDOCX(data)
	case ".xlsx":
		return extractXLSX(data)
	case ".pptx":
		return extractPPTX(data)
	case ".txt", ".md", ".go", ".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".conf", ".log", ".csv", ".tsv",
		".html", ".htm", ".xml", ".css", ".js", ".ts", ".jsx", ".tsx", ".py", ".java", ".c", ".cpp", ".h", ".hpp",
		".rs", ".sh", ".bat", ".ps1", ".sql", ".rb", ".php", ".swift", ".kt", ".scala", ".r", ".lua":
		// Plain text formats, return as-is
		return string(data), nil
	default:
		// Unknown format: return raw string; caller may reject
		return string(data), fmt.Errorf("unsupported file format: %s (supported: .pdf, .docx, .xlsx, .pptx, .txt, .md)", ext)
	}
}

// extractPDF extracts text from a PDF byte slice using ledongthuc/pdf.
func extractPDF(data []byte) (string, error) {
	reader := bytes.NewReader(data)
	r, err := pdf.NewReader(reader, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pdf read: %w", err)
	}

	textReader, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("pdf extract: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, textReader); err != nil {
		return "", fmt.Errorf("pdf read text: %w", err)
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("pdf extract: no text content found (scanned PDF or image-based)")
	}
	return result, nil
}

// extractDOCX extracts text from a .docx file (ZIP containing word/document.xml).
func extractDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("docx open zip: %w", err)
	}

	var docXML []byte
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("docx open xml: %w", err)
			}
			docXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", fmt.Errorf("docx read xml: %w", err)
			}
			break
		}
	}
	if docXML == nil {
		return "", fmt.Errorf("docx: word/document.xml not found in archive")
	}

	return extractXMLText(string(docXML)), nil
}

// extractXLSX extracts text from a .xlsx file (ZIP with shared strings + worksheets).
func extractXLSX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("xlsx open zip: %w", err)
	}

	// Read shared strings (xl/sharedStrings.xml)
	sharedStrings := make(map[int]string)
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("xlsx open shared strings: %w", err)
			}
			xmlData, _ := io.ReadAll(rc)
			rc.Close()
			sharedStrings = parseXLSXSharedStrings(string(xmlData))
			break
		}
	}

	// Read all worksheets and extract cell text
	var parts []string
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			xmlData, _ := io.ReadAll(rc)
			rc.Close()

			sheetText := parseXLSXSheet(string(xmlData), sharedStrings)
			if sheetText != "" {
				parts = append(parts, sheetText)
			}
		}
	}

	result := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if result == "" {
		return "", fmt.Errorf("xlsx: no text content found")
	}
	return result, nil
}

// extractPPTX extracts text from a .pptx file (ZIP with slide XMLs).
func extractPPTX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("pptx open zip: %w", err)
	}

	var parts []string
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			xmlData, _ := io.ReadAll(rc)
			rc.Close()

			slideText := extractPPTXSlideText(string(xmlData))
			if slideText != "" {
				parts = append(parts, slideText)
			}
		}
	}

	result := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if result == "" {
		return "", fmt.Errorf("pptx: no text content found")
	}
	return result, nil
}

// ── XML helpers ──

// extractXMLText extracts text content from XML by finding all text between tags.
// Works for DOCX (w:t tags) and any XML where text is between angle brackets.
func extractXMLText(xmlContent string) string {
	// Simple tag-based extraction: find content between > and <
	var parts []string
	for {
		gt := strings.Index(xmlContent, ">")
		if gt == -1 {
			break
		}
		xmlContent = xmlContent[gt+1:]
		lt := strings.Index(xmlContent, "<")
		if lt == -1 {
			break
		}
		text := strings.TrimSpace(xmlContent[:lt])
		if text != "" {
			parts = append(parts, text)
		}
		xmlContent = xmlContent[lt:]
	}
	return strings.Join(parts, " ")
}

// parseXLSXSharedStrings parses xl/sharedStrings.xml and returns a map of index -> text.
func parseXLSXSharedStrings(xmlContent string) map[int]string {
	result := make(map[int]string)
	type si struct {
		Text string `xml:"t"`
	}
	type sst struct {
		XMLName xml.Name `xml:"sst"`
		Items   []si     `xml:"si"`
	}

	var sstData sst
	if err := xml.Unmarshal([]byte(xmlContent), &sstData); err != nil {
		return result
	}
	for i, item := range sstData.Items {
		result[i] = item.Text
	}
	return result
}

// parseXLSXSheet extracts cell texts from a worksheet XML using shared string references.
func parseXLSXSheet(xmlContent string, sharedStrings map[int]string) string {
	type cell struct {
		Type  string `xml:"t,attr"`
		Value string `xml:"v"`
	}
	type row struct {
		Cells []cell `xml:"c"`
	}
	type sheetData struct {
		Rows []row `xml:"row"`
	}
	type worksheet struct {
		XMLName xml.Name  `xml:"worksheet"`
		Data    sheetData `xml:"sheetData"`
	}

	var ws worksheet
	if err := xml.Unmarshal([]byte(xmlContent), &ws); err != nil {
		return ""
	}

	var parts []string
	for _, row := range ws.Data.Rows {
		var rowTexts []string
		for _, c := range row.Cells {
			var text string
			if c.Type == "s" {
				// Shared string reference
				idx := 0
				fmt.Sscanf(c.Value, "%d", &idx)
				text = sharedStrings[idx]
			} else {
				text = c.Value
			}
			if text != "" {
				rowTexts = append(rowTexts, text)
			}
		}
		if len(rowTexts) > 0 {
			parts = append(parts, strings.Join(rowTexts, "\t"))
		}
	}
	return strings.Join(parts, "\n")
}

// extractPPTXSlideText extracts text content from a PPTX slide XML.
// Text is in a:t elements within a:p (paragraph) elements.
func extractPPTXSlideText(xmlContent string) string {
	type atText struct {
		Text string `xml:",chardata"`
	}
	type aRuns struct {
		Runs []atText `xml:"r"`
	}
	type aParagraph struct {
		Runs  aRuns    `xml:"r"`
		Field atText   `xml:"fld"`
	}
	type spTree struct {
		Paragraphs []aParagraph `xml:"p"`
	}
	type slide struct {
		XMLName xml.Name `xml:"sld"`
		SpTree  spTree   `xml:"cSld>spTree"`
	}

	var sld slide
	if err := xml.Unmarshal([]byte(xmlContent), &sld); err != nil {
		// Fall back to generic text extraction
		return extractXMLText(xmlContent)
	}

	var parts []string
	for _, p := range sld.SpTree.Paragraphs {
		var lineParts []string
		for _, r := range p.Runs.Runs {
			if r.Text != "" {
				lineParts = append(lineParts, r.Text)
			}
		}
		if p.Field.Text != "" {
			lineParts = append(lineParts, p.Field.Text)
		}
		if len(lineParts) > 0 {
			parts = append(parts, strings.Join(lineParts, ""))
		}
	}
	return strings.Join(parts, "\n")
}
