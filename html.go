package main

import (
	"bytes"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// markdown is the shared Markdown -> HTML converter (GitHub flavoured).
var markdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

// pageCSS renders every piece of document text, including code, with a
// sans-serif typeface as required for export and web view.
const pageCSS = `
:root { color-scheme: light dark; }
html, body, body * {
  font-family: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", "Liberation Sans", sans-serif;
}
body {
  max-width: 52rem;
  margin: 2rem auto;
  padding: 0 1rem;
  line-height: 1.6;
  color: #1f2328;
  background: #ffffff;
}
h1, h2 { border-bottom: 1px solid #d0d7de; padding-bottom: .3em; }
a { color: #0969da; }
pre, code, kbd, samp, tt { font-family: inherit; }
pre {
  background: #f6f8fa;
  padding: 1rem;
  border-radius: 6px;
  overflow: auto;
}
code { background: #f6f8fa; padding: .15em .35em; border-radius: 4px; }
pre code { background: none; padding: 0; }
blockquote {
  margin: 0;
  padding: 0 1em;
  color: #59636e;
  border-left: .25em solid #d0d7de;
}
table { border-collapse: collapse; }
th, td { border: 1px solid #d0d7de; padding: .4em .8em; }
tr:nth-child(2n) { background: #f6f8fa; }
img { max-width: 100%; }
hr { border: 0; border-top: 1px solid #d0d7de; }
@media (prefers-color-scheme: dark) {
  body { color: #e6edf3; background: #0d1117; }
  a { color: #4493f8; }
  pre, code, tr:nth-child(2n) { background: #161b22; }
  h1, h2, hr, th, td, blockquote { border-color: #30363d; }
  blockquote { color: #9198a1; }
}
`

// renderHTML converts Markdown source into a complete standalone HTML page.
func renderHTML(title, source string) ([]byte, error) {
	var body bytes.Buffer
	if err := markdown.Convert([]byte(source), &body); err != nil {
		return nil, err
	}
	var page bytes.Buffer
	page.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n")
	page.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	page.WriteString("<title>" + html.EscapeString(title) + "</title>\n")
	page.WriteString("<style>" + pageCSS + "</style>\n</head>\n<body>\n")
	page.Write(body.Bytes())
	page.WriteString("\n</body>\n</html>\n")
	return page.Bytes(), nil
}

// htmlPathFor returns the export path for a Markdown file: same directory,
// same base name, ".html" extension.
func htmlPathFor(mdPath string) string {
	ext := filepath.Ext(mdPath)
	return strings.TrimSuffix(mdPath, ext) + ".html"
}

// exportHTML writes the rendered page next to the Markdown file and returns
// the path of the written file.
func exportHTML(mdPath, source string) (string, error) {
	out := htmlPathFor(mdPath)
	page, err := renderHTML(documentTitle(mdPath), source)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(out, page, 0o644); err != nil {
		return "", err
	}
	return out, nil
}

// writeTempHTML renders the page into a fresh temporary file for "Web View".
func writeTempHTML(title, source string) (string, error) {
	page, err := renderHTML(title, source)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "mdedit-*.html")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(page); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// documentTitle derives a page title from a file path.
func documentTitle(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
