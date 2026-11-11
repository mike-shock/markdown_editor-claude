package main

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// previewDelay debounces live preview re-rendering while typing.
const previewDelay = 150 * time.Millisecond

// editor is the main application window with its state.
type editor struct {
	app         fyne.App
	win         fyne.Window
	previewOnly bool

	entry         *widget.Entry
	editScroll    *container.Scroll
	preview       *widget.RichText
	previewScroll *container.Scroll
	status        *widget.Label
	position      *widget.Label

	path  string // current file; empty for an untitled document
	dirty bool

	syncing     bool        // guards against scroll feedback loops
	renderTimer *time.Timer // pending preview update

	settingsWin fyne.Window // open settings window, if any

	tempMu    sync.Mutex
	tempFiles []string // "Web View" files to delete on exit
}

func newEditor(a fyne.App, previewOnly bool) *editor {
	e := &editor{app: a, previewOnly: previewOnly}
	e.win = a.NewWindow(lang.L("Markdown Editor"))
	e.win.Resize(fyne.NewSize(1100, 720))
	e.win.SetCloseIntercept(e.requestClose)
	e.win.SetMaster() // closing the main window also closes Settings

	e.buildWidgets()
	e.win.SetContent(e.buildLayout())
	e.updateTitle()
	e.renderPreview()
	if !previewOnly {
		e.win.Canvas().Focus(e.entry)
	}
	return e
}

// buildWidgets creates the editor, preview and status widgets.
func (e *editor) buildWidgets() {
	// The entry does its own scrolling only when wrapping is on or Scroll is
	// set. Turning both off lets us wrap it in our own Scroll container,
	// which exposes the Offset needed for synchronous scrolling.
	e.entry = widget.NewMultiLineEntry()
	e.entry.Wrapping = fyne.TextWrapOff
	e.entry.Scroll = container.ScrollNone
	e.entry.TextStyle = fyne.TextStyle{Monospace: true}
	e.entry.SetPlaceHolder(lang.L("Type Markdown here…"))
	e.entry.OnChanged = e.onTextChanged
	e.entry.OnCursorChanged = e.onCursorChanged
	e.editScroll = container.NewScroll(e.entry)
	e.editScroll.OnScrolled = func(fyne.Position) { e.syncScroll(e.editScroll, e.previewScroll) }

	e.preview = widget.NewRichText()
	e.preview.Wrapping = fyne.TextWrapWord
	e.previewScroll = container.NewVScroll(e.preview)
	e.previewScroll.OnScrolled = func(fyne.Position) { e.syncScroll(e.previewScroll, e.editScroll) }

	e.status = widget.NewLabel("")
	e.status.Truncation = fyne.TextTruncateEllipsis
	e.position = widget.NewLabel("")
	e.updatePosition()
}

// buildLayout assembles the toolbar, panels and status bar.
func (e *editor) buildLayout() fyne.CanvasObject {
	openBtn := widget.NewButtonWithIcon(lang.L("Open"), theme.FolderOpenIcon(), e.openDialog)
	exportBtn := widget.NewButtonWithIcon(lang.L("Export HTML"), theme.DocumentIcon(), e.exportHTML)
	webBtn := widget.NewButtonWithIcon(lang.L("Web View"), theme.ComputerIcon(), e.webView)
	settingsBtn := widget.NewButtonWithIcon(lang.L("Settings"), theme.SettingsIcon(), e.showSettings)
	quitBtn := widget.NewButtonWithIcon(lang.L("Quit"), theme.LogoutIcon(), e.quit)

	var toolbar *fyne.Container
	var body fyne.CanvasObject
	if e.previewOnly {
		toolbar = container.NewHBox(openBtn, exportBtn, webBtn,
			widget.NewSeparator(), settingsBtn, quitBtn)
		body = e.previewScroll
	} else {
		newBtn := widget.NewButtonWithIcon(lang.L("New"), theme.DocumentCreateIcon(), e.newDocument)
		saveBtn := widget.NewButtonWithIcon(lang.L("Save"), theme.DocumentSaveIcon(), e.save)
		saveAsBtn := widget.NewButton(lang.L("Save As…"), func() { e.saveAsDialog(nil) })
		toolbar = container.NewHBox(newBtn, openBtn, saveBtn, saveAsBtn, widget.NewSeparator(),
			exportBtn, webBtn, widget.NewSeparator(), settingsBtn, quitBtn)
		body = container.NewHSplit(e.editScroll, e.previewScroll)
	}

	statusBar := container.NewBorder(nil, nil, nil, e.position, e.status)
	return container.NewBorder(toolbar, statusBar, nil, nil, body)
}

// --- document state -------------------------------------------------------

func (e *editor) displayName() string {
	if e.path == "" {
		return lang.L("Untitled")
	}
	return filepath.Base(e.path)
}

func (e *editor) updateTitle() {
	title := lang.L("Markdown Editor") + " — " + e.displayName()
	if e.dirty {
		title += " *"
	}
	e.win.SetTitle(title)
	if e.dirty {
		e.setStatus(lang.L("Modified"))
	}
}

func (e *editor) setStatus(text string) {
	e.status.SetText(text)
}

func (e *editor) setDirty(dirty bool) {
	if e.dirty == dirty {
		return
	}
	e.dirty = dirty
	e.updateTitle()
}

func (e *editor) onTextChanged(string) {
	e.setDirty(true)
	e.schedulePreview()
	e.updatePosition()
}

func (e *editor) onCursorChanged() {
	e.updatePosition()
	e.ensureCursorVisible()
}

func (e *editor) updatePosition() {
	if e.previewOnly {
		return
	}
	e.position.SetText(lang.L("Line {{.Line}}, Column {{.Col}}",
		map[string]any{"Line": e.entry.CursorRow + 1, "Col": e.entry.CursorColumn + 1}))
}

// --- preview --------------------------------------------------------------

func (e *editor) schedulePreview() {
	if e.renderTimer != nil {
		e.renderTimer.Stop()
	}
	e.renderTimer = time.AfterFunc(previewDelay, func() {
		fyne.Do(e.renderPreview)
	})
}

func (e *editor) renderPreview() {
	e.preview.ParseMarkdown(e.entry.Text)
	// The preview height changed: keep it aligned with the editor.
	e.syncScroll(e.editScroll, e.previewScroll)
}

// --- synchronous scrolling -----------------------------------------------

// maxOffset returns how far a scroll container can be scrolled vertically.
func maxOffset(s *container.Scroll) float32 {
	if s.Content == nil {
		return 0
	}
	return s.Content.Size().Height - s.Size().Height
}

// syncScroll moves "to" so that its relative vertical position matches
// "from". Lines of equal height in the editor map proportionally onto the
// rendered preview, which places corresponding content at roughly the same
// place on screen. ScrollToOffset does not fire OnScrolled, and the syncing
// flag guards against re-entrancy for good measure.
func (e *editor) syncScroll(from, to *container.Scroll) {
	if e.previewOnly || e.syncing || from == nil || to == nil {
		return
	}
	e.syncing = true
	defer func() { e.syncing = false }()

	ratio := float32(0)
	if fm := maxOffset(from); fm > 0 {
		ratio = from.Offset.Y / fm
	}
	tm := maxOffset(to)
	if tm < 0 {
		tm = 0
	}
	to.ScrollToOffset(fyne.NewPos(to.Offset.X, ratio*tm))
}

// ensureCursorVisible scrolls the editor so the caret row stays on screen.
// This is normally done by the Entry's internal scroller, which we disabled.
func (e *editor) ensureCursorVisible() {
	if e.previewOnly {
		return
	}
	rows := strings.Count(e.entry.Text, "\n") + 1
	th := e.entry.Theme()
	pad := th.Size(theme.SizeNameInnerPadding)
	rowHeight := (e.entry.MinSize().Height - 2*pad) / float32(rows)
	if rowHeight <= 0 {
		return
	}
	viewH := e.editScroll.Size().Height
	if viewH <= 0 {
		return
	}
	top := pad + float32(e.entry.CursorRow)*rowHeight
	bottom := top + rowHeight + pad

	off := e.editScroll.Offset
	switch {
	case top < off.Y:
		off.Y = top - pad
	case bottom > off.Y+viewH:
		off.Y = bottom - viewH
	default:
		return
	}
	if off.Y < 0 {
		off.Y = 0
	}
	e.editScroll.ScrollToOffset(off)
	e.syncScroll(e.editScroll, e.previewScroll)
}

// --- file operations ------------------------------------------------------

// workingDirURI returns the current working directory as a listable URI so
// file dialogs open there instead of in $HOME.
func workingDirURI() fyne.ListableURI {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	l, err := storage.ListerForURI(storage.NewFileURI(cwd))
	if err != nil {
		return nil
	}
	return l
}

func markdownFilter() storage.FileFilter {
	return storage.NewExtensionFileFilter([]string{".md", ".markdown", ".mdown", ".txt"})
}

// openPath loads a file from disk into the editor.
func (e *editor) openPath(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		e.showError(lang.L("Could not open file {{.Path}}: {{.Error}}",
			map[string]any{"Path": path, "Error": err.Error()}))
		return
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	e.loadDocument(path, string(data))
}

func (e *editor) loadDocument(path, text string) {
	e.path = path
	e.entry.OnChanged = nil // avoid marking the fresh document dirty
	e.entry.SetText(text)
	e.entry.OnChanged = e.onTextChanged
	e.entry.CursorRow, e.entry.CursorColumn = 0, 0
	e.dirty = false
	e.editScroll.ScrollToTop()
	e.previewScroll.ScrollToTop()
	e.renderPreview()
	e.updateTitle()
	e.updatePosition()
	if path == "" {
		e.setStatus(lang.L("New document"))
	} else {
		e.setStatus(lang.L("Opened {{.Path}}", map[string]any{"Path": path}))
	}
}

// newDocument replaces the current document with an empty untitled one,
// asking first if there are unsaved changes.
func (e *editor) newDocument() {
	e.confirmDiscard(func() {
		e.loadDocument("", "")
		e.win.Canvas().Focus(e.entry)
	})
}

// openDialog shows a file chooser rooted at the current working directory.
func (e *editor) openDialog() {
	e.confirmDiscard(func() {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil {
				e.showError(err.Error())
				return
			}
			if r == nil {
				return
			}
			defer r.Close()
			data, err := io.ReadAll(r)
			if err != nil {
				e.showError(lang.L("Could not open file {{.Path}}: {{.Error}}",
					map[string]any{"Path": r.URI().Path(), "Error": err.Error()}))
				return
			}
			e.loadDocument(r.URI().Path(), string(data))
		}, e.win)
		d.SetFilter(markdownFilter())
		if cwd := workingDirURI(); cwd != nil {
			d.SetLocation(cwd)
		}
		d.Resize(e.dialogSize())
		d.Show()
	})
}

// save writes to the current file or asks for a name if there is none.
func (e *editor) save() {
	e.saveThen(nil)
}

// saveThen saves the document (asking for a name if it is untitled) and
// calls next only if saving succeeded. next may be nil.
func (e *editor) saveThen(next func()) {
	if e.path == "" {
		e.saveAsDialog(next)
		return
	}
	if e.saveTo(e.path) && next != nil {
		next()
	}
}

func (e *editor) saveTo(path string) bool {
	if err := os.WriteFile(path, []byte(e.entry.Text), 0o644); err != nil {
		e.showError(lang.L("Could not save file {{.Path}}: {{.Error}}",
			map[string]any{"Path": path, "Error": err.Error()}))
		return false
	}
	e.path = path
	e.dirty = false
	e.updateTitle()
	e.setStatus(lang.L("Saved {{.Path}}", map[string]any{"Path": path}))
	return true
}

// saveAsDialog asks for a file name and saves there; onSaved (may be nil)
// runs after a successful save.
func (e *editor) saveAsDialog(onSaved func()) {
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			e.showError(err.Error())
			return
		}
		if w == nil {
			return
		}
		// Fyne already created the file; write through the provided writer.
		path := w.URI().Path()
		_, werr := w.Write([]byte(e.entry.Text))
		cerr := w.Close()
		if werr == nil {
			werr = cerr
		}
		if werr != nil {
			e.showError(lang.L("Could not save file {{.Path}}: {{.Error}}",
				map[string]any{"Path": path, "Error": werr.Error()}))
			return
		}
		e.path = path
		e.dirty = false
		e.updateTitle()
		e.setStatus(lang.L("Saved {{.Path}}", map[string]any{"Path": path}))
		if onSaved != nil {
			onSaved()
		}
	}, e.win)
	d.SetFilter(markdownFilter())
	if e.path != "" {
		d.SetFileName(filepath.Base(e.path))
		if l, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(e.path))); err == nil {
			d.SetLocation(l)
		}
	} else {
		d.SetFileName("untitled.md")
		if cwd := workingDirURI(); cwd != nil {
			d.SetLocation(cwd)
		}
	}
	d.Resize(e.dialogSize())
	d.Show()
}

// exportHTML writes <name>.html next to the current Markdown file.
func (e *editor) exportHTML() {
	if e.path == "" {
		dialog.ShowInformation(lang.L("Export HTML"),
			lang.L("Save the document before exporting to HTML."), e.win)
		return
	}
	out, err := exportHTML(e.path, e.entry.Text)
	if err != nil {
		e.showError(lang.L("Could not export HTML: {{.Error}}", map[string]any{"Error": err.Error()}))
		return
	}
	msg := lang.L("Exported to {{.Path}}", map[string]any{"Path": out})
	e.setStatus(msg)
	dialog.ShowInformation(lang.L("Export complete"), msg, e.win)
}

// webView renders the document into a temporary HTML file and opens it in
// the system browser. The file is removed when the editor exits.
func (e *editor) webView() {
	title := lang.L("Untitled")
	if e.path != "" {
		title = documentTitle(e.path)
	}
	tmp, err := writeTempHTML(title, e.entry.Text)
	if err != nil {
		e.showError(lang.L("Could not create temporary file: {{.Error}}", map[string]any{"Error": err.Error()}))
		return
	}
	e.tempMu.Lock()
	e.tempFiles = append(e.tempFiles, tmp)
	e.tempMu.Unlock()

	u := &url.URL{Scheme: "file", Path: filepath.ToSlash(tmp)}
	if err := e.app.OpenURL(u); err != nil {
		e.showError(lang.L("Could not open browser: {{.Error}}", map[string]any{"Error": err.Error()}))
		return
	}
	e.setStatus(lang.L("Opened in browser: {{.Path}}", map[string]any{"Path": tmp}))
}

// cleanup deletes temporary files created by webView. Safe to call twice.
func (e *editor) cleanup() {
	e.tempMu.Lock()
	files := e.tempFiles
	e.tempFiles = nil
	e.tempMu.Unlock()
	for _, f := range files {
		_ = os.Remove(f)
	}
}

// --- helpers --------------------------------------------------------------

func (e *editor) dialogSize() fyne.Size {
	s := e.win.Canvas().Size()
	return fyne.NewSize(s.Width*0.85, s.Height*0.85)
}

func (e *editor) showError(msg string) {
	e.setStatus(msg)
	dialog.ShowInformation(lang.L("Error"), msg, e.win)
}

// confirmDiscard runs next immediately if nothing would be lost. Otherwise
// it asks whether to save the changes first, discard them, or cancel.
func (e *editor) confirmDiscard(next func()) {
	if !e.dirty || e.previewOnly {
		next()
		return
	}
	var d *dialog.CustomDialog
	saveBtn := widget.NewButtonWithIcon(lang.L("Save"), theme.DocumentSaveIcon(), func() {
		d.Hide()
		e.saveThen(next)
	})
	saveBtn.Importance = widget.HighImportance
	discardBtn := widget.NewButtonWithIcon(lang.L("Discard"), theme.DeleteIcon(), func() {
		d.Hide()
		next()
	})
	cancelBtn := widget.NewButtonWithIcon(lang.L("Cancel"), theme.CancelIcon(), func() {
		d.Hide()
	})
	msg := widget.NewLabel(lang.L("Save changes to {{.Name}} before continuing?",
		map[string]any{"Name": e.displayName()}))
	d = dialog.NewCustomWithoutButtons(lang.L("Unsaved changes"), msg, e.win)
	d.SetButtons([]fyne.CanvasObject{cancelBtn, discardBtn, saveBtn})
	d.Show()
}

// requestClose handles the window close button.
func (e *editor) requestClose() {
	e.quit()
}

// quit exits the application after checking for unsaved changes.
func (e *editor) quit() {
	e.confirmDiscard(func() {
		e.cleanup()
		if e.settingsWin != nil {
			e.settingsWin.Close()
		}
		e.app.Quit()
	})
}
