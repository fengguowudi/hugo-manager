package main

import (
	"testing"

	"fyne.io/fyne/v2/widget"
)

func TestMarkdownHelpers(t *testing.T) {
	e := widget.NewMultiLineEntry()
	e.SetText("hello\n世界")
	e.CursorRow, e.CursorColumn = 1, 1
	if got := cursorOffset(e); got != 7 { // 5 + \n + 1 rune
		t.Fatalf("cursorOffset = %d, want 7", got)
	}

	mdWrap(e, "**", "**", "粗体文本")
	if e.Text != "hello\n世**粗体文本**界" {
		t.Fatalf("mdWrap got %q", e.Text)
	}

	mdHeading(e, 2)
	if e.Text != "hello\n## 世**粗体文本**界" {
		t.Fatalf("mdHeading got %q", e.Text)
	}

	e.SetText("line1\nline2")
	e.CursorRow, e.CursorColumn = 1, 3
	mdLinePrefix(e, "> ")
	if e.Text != "line1\n> line2" {
		t.Fatalf("mdLinePrefix got %q", e.Text)
	}

	e.SetText("ab")
	e.CursorRow, e.CursorColumn = 0, 1
	mdInsertBlock(e, "\n| a |\n")
	if e.Text != "a\n| a |\nb" {
		t.Fatalf("mdInsertBlock got %q", e.Text)
	}

	setEntryCursor(e, 0)
	if e.CursorRow != 0 || e.CursorColumn != 0 {
		t.Fatalf("setEntryCursor got row=%d col=%d", e.CursorRow, e.CursorColumn)
	}
}
