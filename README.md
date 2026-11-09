# mdedit — Markdown editor and viewer (Go + Fyne)

A desktop Markdown editor with live preview, HTML export, browser preview,
synchronised scrolling and a localised UI (English, Russian, Japanese).

## Build and run

Requires Go 1.24+ and the usual Fyne build dependencies on Linux
(`gcc`, X11/OpenGL headers: `libgl1-mesa-dev xorg-dev` on Debian/Ubuntu).

```sh
go build -o mdedit .
./mdedit                      # empty document
./mdedit notes.md             # open a file for editing
./mdedit --preview notes.md   # preview-only mode (no editor panel)
./mdedit --lang ru notes.md   # force the UI language (en, ru, ja)
./mdedit --help
```

Options may be placed before or after the file name.

## Features

| Button / action  | What it does |
|------------------|--------------|
| **Open**         | File chooser that starts in the *current working directory*, not `$HOME`. |
| **Save**         | Writes the current file; asks for a name if the document is untitled. |
| **Save As…**     | Save under a new name (dialog starts in the file's directory or the CWD). |
| **Export HTML**  | Writes `<name>.html` next to the `.md` file (GitHub-flavoured Markdown via goldmark). |
| **Web View**     | Renders the document to a temporary `.html` file and opens it in the default browser. Temporary files are deleted when the editor exits. |

* Live preview updates while typing (debounced).
* Editor (left) and preview (right) scroll together in both directions; the
  relative scroll position of one panel is mirrored in the other, so the
  corresponding content stays at roughly the same height.
* Exported / web-view HTML styles all document text, including code, with a
  sans-serif font stack and supports light and dark colour schemes.
* Unsaved changes are confirmed before opening another file or closing.

## Localisation

All captions and messages go through Fyne's `lang` package (`lang.L(...)`).
Translations live in `translations/{en,ru,ja}.json` and are embedded with
`//go:embed` and registered via `lang.AddTranslationsFS`. The language is
picked from the system locale (`LANGUAGE`, `LC_ALL`, `LC_MESSAGES`, `LANG`)
or overridden with `--lang`. To add a language, drop another `xx.json`
with the same keys into `translations/`.

## Tests

```sh
go test ./...
```

Tests cover command-line parsing, HTML rendering/export, temp-file creation,
completeness of the translation files and runtime localisation.

## Layout

* `main.go` — argument parsing, translation loading, application start-up
* `editor.go` — window, widgets, file operations, synchronised scrolling
* `html.go` — Markdown → HTML conversion, export and temporary files
* `translations/` — UI strings for en / ru / ja
* `sample.md` — a document to try the editor with
