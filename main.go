// Command mdedit is a desktop Markdown editor and viewer built with Fyne.
//
// Usage:
//
//	mdedit [--preview] [--lang en|ru|ja] [file.md]
package main

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/lang"
)

//go:embed translations
var translations embed.FS

// options holds the parsed command line.
type options struct {
	previewOnly bool
	language    string
	path        string
	help        bool
}

// parseArgs parses the command line. Unlike the standard flag package it
// accepts options both before and after the file name.
func parseArgs(args []string) (options, error) {
	var o options
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--preview" || a == "-preview" || a == "-p":
			o.previewOnly = true
		case a == "--help" || a == "-help" || a == "-h":
			o.help = true
		case strings.HasPrefix(a, "--lang=") || strings.HasPrefix(a, "-lang="):
			o.language = a[strings.Index(a, "=")+1:]
		case a == "--lang" || a == "-lang" || a == "-l":
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s requires a value", a)
			}
			i++
			o.language = args[i]
		case strings.HasPrefix(a, "-") && a != "-":
			return o, fmt.Errorf("unknown option %q", a)
		default:
			if o.path != "" {
				return o, fmt.Errorf("only one file name may be given")
			}
			o.path = a
		}
	}
	if o.previewOnly && o.path == "" && !o.help {
		return o, fmt.Errorf("--preview requires a file name")
	}
	return o, nil
}

func usage() string {
	return lang.L("Usage: mdedit [--preview] [--lang en|ru|ja] [file.md]") + "\n\n" +
		"  --preview   " + lang.L("Preview-only mode: hide the editor and show the rendered Markdown") + "\n" +
		"  --lang XX   " + lang.L("Override the user interface language (en, ru, ja)") + "\n" +
		"  -h, --help  " + lang.L("Show this help") + "\n"
}

// loadTranslations registers the embedded translation bundles. If a language
// override is given it is applied through the LANGUAGE environment variable,
// which is what Fyne (via go-locale) consults when picking the UI locale.
func loadTranslations(override string) {
	if override != "" {
		_ = os.Setenv("LANGUAGE", override)
	}
	if err := lang.AddTranslationsFS(translations, "translations"); err != nil {
		fyne.LogError("loading translations", err)
	}
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	loadTranslations(opts.language)
	if err != nil {
		fmt.Fprintln(os.Stderr, lang.L("Error")+": "+err.Error())
		fmt.Fprint(os.Stderr, usage())
		os.Exit(2)
	}
	if opts.help {
		fmt.Print(usage())
		return
	}

	a := app.NewWithID("io.github.mdedit")
	ed := newEditor(a, opts.previewOnly)
	if opts.path != "" {
		ed.openPath(opts.path)
	}
	// Remove temporary "Web View" files however the application ends.
	a.Lifecycle().SetOnStopped(ed.cleanup)
	defer ed.cleanup()

	ed.win.ShowAndRun()
}
