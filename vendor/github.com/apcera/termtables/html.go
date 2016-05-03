// Copyright 2013 Apcera Inc. All rights reserved.

package termtables

import (
	"bytes"
	"fmt"
	"html"
	"strings"
)

type titleStyle int

const (
	TitleAsCaption titleStyle = iota
	TitleAsThSpan
)

// htmlStyleRules defines attributes which we can use, and might be set on a
// table by accessors, to influence the type of HTML which is output.
type htmlStyleRules struct {
	title titleStyle
}

// HTML returns an HTML representations of the contents of one row of a table.
func (r *Row) HTML(tag string, style *renderStyle) string {
	attrs := make([]string, len(r.cells))
	elems := make([]string, len(r.cells))
	for i := range r.cells {
		if r.cells[i].alignment != nil {
			switch *r.cells[i].alignment {
			case AlignLeft:
				attrs[i] = " align='left'"
			case AlignCenter:
				attrs[i] = " align='center'"
			case AlignRight:
				attrs[i] = " align='right'"
			}
		}
		elems[i] = html.EscapeString(strings.TrimSpace(r.cells[i].Render(style)))
	}
	// WAG as to max capacity, plus a bit
	buf := bytes.NewBuffer(make([]byte, 0, 8192))
	buf.WriteString("<tr>")
	for i := range elems {
		fmt.Fprintf(buf, "<%s%s>%s</%s>", tag, attrs[i], elems[i], tag)
	}
	buf.WriteString("</tr>\n")
	return buf.String()
}

func generateHtmlTitleRow(title interface{}, t *Table, style *renderStyle) string {
	elContent := html.EscapeString(
		strings.TrimSpace(CreateCell(t.title, &CellStyle{}).Render(style)),
	)

	switch style.htmlRules.title {
	case TitleAsCaption:
		return "<caption>" + elContent + "</caption>\n"
	case TitleAsThSpan:
		return fmt.Sprintf("<tr><th style=\"text-align: center\" colspan=\"%d\">%s</th></tr>\n",
			style.columns, elContent)
	default:
		return "<!-- " + elContent + " -->"
	}
}

// RenderHTML returns a string representation of a the table, suitable for
// inclusion as HTML elsewhere.  Primary use-case controlling layout style
// is for inclusion into Markdown documents, documenting normal table use.
// Thus we leave the padding in place to have columns align when viewed as
// plain text and rely upon HTML ignoring extra whitespace.
func (t *Table) RenderHTML() (buffer string) {
	// elements is already populated with row data

	// generate the runtime style
	style := createRenderStyle(t)
	style.PaddingLeft = 0
	style.PaddingRight = 0

	// TODO: control CSS styles to suppress border based upon t.Style.SkipBorder
	rowsText := make([]string, 0, len(t.elements)+6)

	if t.title != nil || t.headers != nil {
		rowsText = append(rowsText, "<thead>\n")
		if t.title != nil {
			rowsText = append(rowsText, generateHtmlTitleRow(t.title, t, style))
		}
		if t.headers != nil {
			rowsText = append(rowsText, CreateRow(t.headers).HTML("th", style))
		}
		rowsText = append(rowsText, "</thead>\n")
	}

	rowsText = append(rowsText, "<tbody>\n")
	// loop over the elements and render them
	for i := range t.elements {
		if row, ok := t.elements[i].(*Row); ok {
			rowsText = append(rowsText, row.HTML("td", style))
		} else {
			rowsText = append(rowsText, fmt.Sprintf("<!-- unable to render line %d, unhandled type -->\n", i))
		}
	}
	rowsText = append(rowsText, "</tbody>\n")

	return "<table class=\"termtable\">\n" + strings.Join(rowsText, "") + "</table>\n"
}
