package main

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestSettingsNormalized(t *testing.T) {
	cases := []struct{ in, want settings }{
		{settings{}, defaultSettings()},
		{settings{Theme: "neon", FontSize: 9}, settings{themeSystem, defaultFontSize}},
		{settings{Theme: themeDark, FontSize: 19}, settings{themeDark, defaultFontSize}},
		{settings{Theme: themeLight, FontSize: 10}, settings{themeLight, 10}},
		{settings{Theme: themeDark, FontSize: 18}, settings{themeDark, 18}},
	}
	for _, c := range cases {
		if got := c.in.normalized(); got != c.want {
			t.Errorf("%+v.normalized() = %+v, want %+v", c.in, got, c.want)
		}
	}
	if d := defaultSettings(); d.FontSize != 14 || d.Theme != themeSystem {
		t.Errorf("defaults = %+v", d)
	}
}

func TestSettingsPersist(t *testing.T) {
	a := test.NewTempApp(t)
	if got := loadSettings(a.Preferences()); got != defaultSettings() {
		t.Errorf("fresh preferences = %+v", got)
	}
	applySettings(a, settings{Theme: themeDark, FontSize: 17})
	if got := loadSettings(a.Preferences()); got != (settings{themeDark, 17}) {
		t.Errorf("after apply = %+v", got)
	}
	if got := a.Settings().Theme().Size(theme.SizeNameText); got != 17 {
		t.Errorf("applied text size = %v", got)
	}
}

func TestAppTheme(t *testing.T) {
	def := theme.DefaultTheme()
	dark := newAppTheme(settings{Theme: themeDark, FontSize: 14})
	light := newAppTheme(settings{Theme: themeLight, FontSize: 14})
	sys := newAppTheme(settings{Theme: themeSystem, FontSize: 14})
	bg := theme.ColorNameBackground
	if dark.Color(bg, theme.VariantLight) != def.Color(bg, theme.VariantDark) {
		t.Error("dark mode does not force the dark variant")
	}
	if light.Color(bg, theme.VariantDark) != def.Color(bg, theme.VariantLight) {
		t.Error("light mode does not force the light variant")
	}
	if sys.Color(bg, theme.VariantDark) != def.Color(bg, theme.VariantDark) {
		t.Error("system mode should follow the requested variant")
	}

	big := newAppTheme(settings{Theme: themeSystem, FontSize: 18})
	if got := big.Size(theme.SizeNameText); got != 18 {
		t.Errorf("text size = %v, want 18", got)
	}
	wantHeading := def.Size(theme.SizeNameHeadingText) * 18 / 14
	if got := big.Size(theme.SizeNameHeadingText); got != wantHeading {
		t.Errorf("heading size = %v, want %v", got, wantHeading)
	}
	if got, want := big.Size(theme.SizeNamePadding), def.Size(theme.SizeNamePadding); got != want {
		t.Errorf("padding should not scale: %v != %v", got, want)
	}
}

func TestNewDocument(t *testing.T) {
	a := test.NewTempApp(t)
	e := newEditor(a, false)
	e.loadDocument("/tmp/x.md", "# hello")
	e.newDocument() // not dirty: no confirmation needed
	if e.path != "" || e.entry.Text != "" || e.dirty {
		t.Errorf("after New: path=%q text=%q dirty=%v", e.path, e.entry.Text, e.dirty)
	}

	e.entry.SetText("changed")
	if !e.dirty {
		t.Fatal("editing should mark the document dirty")
	}
	e.newDocument() // dirty: must ask instead of discarding silently
	if e.entry.Text != "changed" {
		t.Error("New discarded unsaved changes without confirmation")
	}
}

func TestSettingsWindowSingleton(t *testing.T) {
	a := test.NewTempApp(t)
	e := newEditor(a, false)
	e.showSettings()
	first := e.settingsWin
	if first == nil {
		t.Fatal("settings window not opened")
	}
	e.showSettings()
	if e.settingsWin != first {
		t.Error("second Settings click opened another window")
	}
	first.Close()
	if e.settingsWin != nil {
		t.Error("settingsWin not cleared on close")
	}
}
