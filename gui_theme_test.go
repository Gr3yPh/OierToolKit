package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKDEFontFamilyFromSetting(t *testing.T) {
	if got := kdeFontFamilyFromSetting("Noto Serif CJK SC,11,-1,5,400"); got != "Noto Serif CJK SC" {
		t.Fatalf("font family = %q", got)
	}
	if got := kdeFontFamilyFromSetting("  "); got != "" {
		t.Fatalf("empty setting produced family %q", got)
	}
}

func TestFontHasThemeVariants(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "Example-Regular.ttf")
	for _, path := range []string{regular} {
		if err := os.WriteFile(path, []byte("font"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if fontHasThemeVariants(regular) {
		t.Fatal("regular-only font should not be used as FYNE_FONT")
	}
	for _, variant := range []string{"Bold", "Italic", "BoldItalic"} {
		path := filepath.Join(dir, "Example-"+variant+".ttf")
		if err := os.WriteFile(path, []byte("font"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if !fontHasThemeVariants(regular) {
		t.Fatal("font with all Fyne variants was rejected")
	}
}

func TestApplyFontEnvUsesCompatibleFont(t *testing.T) {
	t.Setenv("FYNE_FONT", "")
	t.Setenv("FYNE_FONT_MONOSPACE", "")
	applyFontEnv()
	if path := os.Getenv("FYNE_FONT"); path != "" && !fontHasThemeVariants(path) {
		t.Fatalf("FYNE_FONT points to a font with missing theme variants: %s", path)
	}
}
