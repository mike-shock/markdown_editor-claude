package main

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Theme modes stored in the preferences.
const (
	themeSystem = "system"
	themeLight  = "light"
	themeDark   = "dark"
)

// Font size limits (in points; Fyne's default text size is 14).
const (
	minFontSize     = 10
	maxFontSize     = 18
	defaultFontSize = 14
)

// Preference keys.
const (
	prefTheme    = "theme"
	prefFontSize = "fontSize"
)

// settings is the user-configurable appearance of the application.
type settings struct {
	Theme    string // themeSystem, themeLight or themeDark
	FontSize int    // minFontSize..maxFontSize
}

func defaultSettings() settings {
	return settings{Theme: themeSystem, FontSize: defaultFontSize}
}

// normalized replaces invalid values with the defaults.
func (s settings) normalized() settings {
	switch s.Theme {
	case themeSystem, themeLight, themeDark:
	default:
		s.Theme = themeSystem
	}
	if s.FontSize < minFontSize || s.FontSize > maxFontSize {
		s.FontSize = defaultFontSize
	}
	return s
}

func loadSettings(p fyne.Preferences) settings {
	return settings{
		Theme:    p.StringWithFallback(prefTheme, themeSystem),
		FontSize: p.IntWithFallback(prefFontSize, defaultFontSize),
	}.normalized()
}

func saveSettings(p fyne.Preferences, s settings) {
	s = s.normalized()
	p.SetString(prefTheme, s.Theme)
	p.SetInt(prefFontSize, s.FontSize)
}

// appTheme wraps the default Fyne theme, forcing a light or dark variant
// if requested and scaling all text sizes to the chosen font size.
type appTheme struct {
	base     fyne.Theme
	mode     string
	fontSize int
}

func newAppTheme(s settings) *appTheme {
	s = s.normalized()
	return &appTheme{base: theme.DefaultTheme(), mode: s.Theme, fontSize: s.FontSize}
}

func (t *appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch t.mode {
	case themeLight:
		v = theme.VariantLight
	case themeDark:
		v = theme.VariantDark
	}
	return t.base.Color(n, v)
}

func (t *appTheme) Font(s fyne.TextStyle) fyne.Resource     { return t.base.Font(s) }
func (t *appTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.base.Icon(n) }

func (t *appTheme) Size(n fyne.ThemeSizeName) float32 {
	size := t.base.Size(n)
	switch n {
	case theme.SizeNameText, theme.SizeNameHeadingText,
		theme.SizeNameSubHeadingText, theme.SizeNameCaptionText:
		return size * float32(t.fontSize) / defaultFontSize
	}
	return size
}

// applySettings installs the theme built from s and persists s.
func applySettings(a fyne.App, s settings) {
	s = s.normalized()
	saveSettings(a.Preferences(), s)
	a.Settings().SetTheme(newAppTheme(s))
}

// showSettings opens the settings window, or focuses it if already open.
// Changes take effect immediately and are stored in the app preferences.
func (e *editor) showSettings() {
	if e.settingsWin != nil {
		e.settingsWin.RequestFocus()
		return
	}
	cur := loadSettings(e.app.Preferences())

	// Theme: display (translated) labels map to stored values.
	modes := []string{themeSystem, themeLight, themeDark}
	labels := []string{lang.L("System"), lang.L("Light"), lang.L("Dark")}
	labelOf := func(mode string) string {
		for i, m := range modes {
			if m == mode {
				return labels[i]
			}
		}
		return labels[0]
	}

	// Font sizes.
	var sizeLabels []string
	for s := minFontSize; s <= maxFontSize; s++ {
		sizeLabels = append(sizeLabels, fontSizeLabel(s))
	}

	themeRadio := widget.NewRadioGroup(labels, nil)
	themeRadio.Horizontal = true
	themeRadio.Required = true
	sizeSelect := widget.NewSelect(sizeLabels, nil)

	setControls := func(s settings) {
		themeRadio.SetSelected(labelOf(s.Theme))
		sizeSelect.SetSelected(fontSizeLabel(s.FontSize))
	}
	setControls(cur)

	themeRadio.OnChanged = func(label string) {
		for i, l := range labels {
			if l == label && cur.Theme != modes[i] {
				cur.Theme = modes[i]
				applySettings(e.app, cur)
			}
		}
	}
	sizeSelect.OnChanged = func(label string) {
		size := minFontSize + sizeSelect.SelectedIndex()
		if size >= minFontSize && size != cur.FontSize {
			cur.FontSize = size
			applySettings(e.app, cur)
		}
	}

	w := e.app.NewWindow(lang.L("Settings"))
	e.settingsWin = w
	w.SetOnClosed(func() { e.settingsWin = nil })

	form := widget.NewForm(
		widget.NewFormItem(lang.L("Theme"), themeRadio),
		widget.NewFormItem(lang.L("Font size"), sizeSelect),
	)
	resetBtn := widget.NewButton(lang.L("Restore defaults"), func() {
		cur = defaultSettings()
		applySettings(e.app, cur)
		setControls(cur)
	})
	closeBtn := widget.NewButton(lang.L("Close"), w.Close)
	closeBtn.Importance = widget.HighImportance
	buttons := container.NewHBox(resetBtn, layout.NewSpacer(), closeBtn)

	w.SetContent(container.NewPadded(container.NewBorder(nil, buttons, nil, nil, form)))
	w.Resize(fyne.NewSize(460, w.Content().MinSize().Height))
	w.CenterOnScreen()
	w.Show()
}

func fontSizeLabel(size int) string {
	return lang.L("{{.Size}} pt", map[string]any{"Size": strconv.Itoa(size)})
}
