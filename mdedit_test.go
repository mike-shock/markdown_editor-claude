package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/lang"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		args    []string
		want    options
		wantErr bool
	}{
		{nil, options{}, false},
		{[]string{"a.md"}, options{path: "a.md"}, false},
		{[]string{"--preview", "a.md"}, options{previewOnly: true, path: "a.md"}, false},
		{[]string{"a.md", "--preview"}, options{previewOnly: true, path: "a.md"}, false},
		{[]string{"--lang", "ru", "a.md"}, options{language: "ru", path: "a.md"}, false},
		{[]string{"--lang=ja"}, options{language: "ja"}, false},
		{[]string{"-h"}, options{help: true}, false},
		{[]string{"--preview"}, options{}, true},
		{[]string{"--bogus"}, options{}, true},
		{[]string{"a.md", "b.md"}, options{}, true},
		{[]string{"--lang"}, options{}, true},
	}
	for _, c := range cases {
		got, err := parseArgs(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("parseArgs(%v) error = %v, wantErr %v", c.args, err, c.wantErr)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("parseArgs(%v) = %+v, want %+v", c.args, got, c.want)
		}
	}
}

func TestRenderHTML(t *testing.T) {
	page, err := renderHTML(`A <b> "title"`, "# Hello\n\nSome *text* and `code`.\n\n| a | b |\n|---|---|\n| 1 | 2 |\n")
	if err != nil {
		t.Fatal(err)
	}
	s := string(page)
	for _, want := range []string{
		"<!DOCTYPE html>",
		`<meta charset="utf-8">`,
		"<title>A &lt;b&gt; &#34;title&#34;</title>",
		"sans-serif",
		"pre, code, kbd, samp, tt { font-family: inherit; }",
		`<h1 id="hello">Hello</h1>`,
		"<em>text</em>",
		"<code>code</code>",
		"<table>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered page lacks %q", want)
		}
	}
}

func TestHTMLPathFor(t *testing.T) {
	cases := map[string]string{
		"/tmp/doc.md":       "/tmp/doc.html",
		"/tmp/doc.markdown": "/tmp/doc.html",
		"notes":             "notes.html",
		"/a/b.c/x.md":       "/a/b.c/x.html",
	}
	for in, want := range cases {
		if got := htmlPathFor(in); got != want {
			t.Errorf("htmlPathFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportAndTemp(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(md, []byte("# T\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exportHTML(md, "# T\n")
	if err != nil {
		t.Fatal(err)
	}
	if out != filepath.Join(dir, "readme.html") {
		t.Errorf("export path = %q", out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("exported file missing: %v", err)
	}

	tmp, err := writeTempHTML("x", "hi")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)
	if !strings.HasSuffix(tmp, ".html") {
		t.Errorf("temp file %q should end with .html", tmp)
	}
	data, _ := os.ReadFile(tmp)
	if !strings.Contains(string(data), "<p>hi</p>") {
		t.Errorf("temp file content unexpected: %s", data)
	}
}

// TestTranslationsComplete checks every key in en.json has a translation in
// the other languages and that no language carries unknown keys.
func TestTranslationsComplete(t *testing.T) {
	load := func(name string) map[string]string {
		data, err := translations.ReadFile("translations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return m
	}
	en := load("en.json")
	if len(en) == 0 {
		t.Fatal("en.json is empty")
	}
	for _, name := range []string{"ru.json", "ja.json"} {
		m := load(name)
		for k := range en {
			if v, ok := m[k]; !ok {
				t.Errorf("%s: missing key %q", name, k)
			} else if v == "" {
				t.Errorf("%s: empty translation for %q", name, k)
			}
		}
		for k := range m {
			if _, ok := en[k]; !ok {
				t.Errorf("%s: unknown key %q", name, k)
			}
		}
	}
}

func TestLocalizeRussian(t *testing.T) {
	t.Setenv("LANGUAGE", "ru")
	loadTranslations("ru")
	if got := lang.L("Web View"); got != "Веб-просмотр" {
		t.Errorf("lang.L(Web View) = %q", got)
	}
	got := lang.L("Saved {{.Path}}", map[string]any{"Path": "/x.md"})
	if got != "Сохранено: /x.md" {
		t.Errorf("templated translation = %q", got)
	}
}
