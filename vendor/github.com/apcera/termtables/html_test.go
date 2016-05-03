// Copyright 2013 Apcera Inc. All rights reserved.

package termtables

import (
	"testing"
)

func TestCreateTableHTML(t *testing.T) {
	expected := "<table class=\"termtable\">\n" +
		"<thead>\n" +
		"<tr><th>Name</th><th>Value</th></tr>\n" +
		"</thead>\n" +
		"<tbody>\n" +
		"<tr><td>hey</td><td>you</td></tr>\n" +
		"<tr><td>ken</td><td>1234</td></tr>\n" +
		"<tr><td>derek</td><td>3.14</td></tr>\n" +
		"<tr><td>derek too</td><td>3.15</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()

	table.AddHeaders("Name", "Value")
	table.AddRow("hey", "you")
	table.AddRow("ken", 1234)
	table.AddRow("derek", 3.14)
	table.AddRow("derek too", 3.1456788)

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}

func TestTableWithHeaderHTML(t *testing.T) {
	expected := "<table class=\"termtable\">\n" +
		"<thead>\n" +
		"<caption>Example</caption>\n" +
		"<tr><th>Name</th><th>Value</th></tr>\n" +
		"</thead>\n" +
		"<tbody>\n" +
		"<tr><td>hey</td><td>you</td></tr>\n" +
		"<tr><td>ken</td><td>1234</td></tr>\n" +
		"<tr><td>derek</td><td>3.14</td></tr>\n" +
		"<tr><td>derek too</td><td>3.15</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()

	table.AddTitle("Example")
	table.AddHeaders("Name", "Value")
	table.AddRow("hey", "you")
	table.AddRow("ken", 1234)
	table.AddRow("derek", 3.14)
	table.AddRow("derek too", 3.1456788)

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}

func TestTableTitleWidthAdjustsHTML(t *testing.T) {
	expected := "<table class=\"termtable\">\n" +
		"<thead>\n" +
		"<caption>Example My Foo Bar&#39;d Test</caption>\n" +
		"<tr><th>Name</th><th>Value</th></tr>\n" +
		"</thead>\n" +
		"<tbody>\n" +
		"<tr><td>hey</td><td>you</td></tr>\n" +
		"<tr><td>ken</td><td>1234</td></tr>\n" +
		"<tr><td>derek</td><td>3.14</td></tr>\n" +
		"<tr><td>derek too</td><td>3.15</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()

	table.AddTitle("Example My Foo Bar'd Test")
	table.AddHeaders("Name", "Value")
	table.AddRow("hey", "you")
	table.AddRow("ken", 1234)
	table.AddRow("derek", 3.14)
	table.AddRow("derek too", 3.1456788)

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}

func TestTableWithNoHeadersHTML(t *testing.T) {
	expected := "<table class=\"termtable\">\n" +
		"<tbody>\n" +
		"<tr><td>hey</td><td>you</td></tr>\n" +
		"<tr><td>ken</td><td>1234</td></tr>\n" +
		"<tr><td>derek</td><td>3.14</td></tr>\n" +
		"<tr><td>derek too</td><td>3.15</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()

	table.AddRow("hey", "you")
	table.AddRow("ken", 1234)
	table.AddRow("derek", 3.14)
	table.AddRow("derek too", 3.1456788)

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}

func TestTableUnicodeWidthsHTML(t *testing.T) {
	expected := "<table class=\"termtable\">\n" +
		"<thead>\n" +
		"<tr><th>Name</th><th>Cost</th></tr>\n" +
		"</thead>\n" +
		"<tbody>\n" +
		"<tr><td>Currency</td><td>¤10</td></tr>\n" +
		"<tr><td>US Dollar</td><td>$30</td></tr>\n" +
		"<tr><td>Euro</td><td>€27</td></tr>\n" +
		"<tr><td>Thai</td><td>฿70</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()
	table.AddHeaders("Name", "Cost")
	table.AddRow("Currency", "¤10")
	table.AddRow("US Dollar", "$30")
	table.AddRow("Euro", "€27")
	table.AddRow("Thai", "฿70")

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}

func TestTableWithAlignment(t *testing.T) {
	expected := "<table class=\"termtable\">\n" +
		"<thead>\n" +
		"<tr><th>Foo</th><th>Bar</th></tr>\n" +
		"</thead>\n" +
		"<tbody>\n" +
		"<tr><td>humpty</td><td>dumpty</td></tr>\n" +
		"<tr><td align='right'>r</td><td>&lt;- on right</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()
	table.AddHeaders("Foo", "Bar")
	table.AddRow("humpty", "dumpty")
	table.AddRow(CreateCell("r", &CellStyle{Alignment: AlignRight}), "<- on right")

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}

func TestTableAfterSetAlign(t *testing.T) {
	expected := "<table class=\"termtable\">\n" +
		"<thead>\n" +
		"<tr><th>Alphabetical</th><th>Num</th></tr>\n" +
		"</thead>\n" +
		"<tbody>\n" +
		"<tr><td align='right'>alfa</td><td>1</td></tr>\n" +
		"<tr><td align='right'>bravo</td><td>2</td></tr>\n" +
		"<tr><td align='right'>charlie</td><td>3</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()
	table.AddHeaders("Alphabetical", "Num")
	table.AddRow("alfa", 1)
	table.AddRow("bravo", 2)
	table.AddRow("charlie", 3)
	table.SetAlign(AlignRight, 1)

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}

func TestTableWithAltTitleStyle(t *testing.T) {
	expected := "" +
		"<table class=\"termtable\">\n" +
		"<thead>\n" +
		"<tr><th style=\"text-align: center\" colspan=\"3\">Metasyntactic</th></tr>\n" +
		"<tr><th>Foo</th><th>Bar</th><th>Baz</th></tr>\n" +
		"</thead>\n" +
		"<tbody>\n" +
		"<tr><td>a</td><td>b</td><td>c</td></tr>\n" +
		"<tr><td>α</td><td>β</td><td>γ</td></tr>\n" +
		"</tbody>\n" +
		"</table>\n"

	table := CreateTable()
	table.SetModeHTML()
	table.SetHTMLStyleTitle(TitleAsThSpan)
	table.AddTitle("Metasyntactic")
	table.AddHeaders("Foo", "Bar", "Baz")
	table.AddRow("a", "b", "c")
	table.AddRow("α", "β", "γ")

	output := table.Render()
	if output != expected {
		t.Fatal(DisplayFailedOutput(output, expected))
	}
}
