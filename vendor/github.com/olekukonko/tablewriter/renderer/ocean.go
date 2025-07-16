package renderer

import (
	"io"
	"strings"

	"github.com/olekukonko/ll"
	"github.com/olekukonko/tablewriter/tw"
)

// OceanConfig defines configuration specific to the Ocean renderer.
type OceanConfig struct {
}

// Ocean is a streaming table renderer that writes ASCII tables.
type Ocean struct {
	config                tw.Rendition
	oceanConfig           OceanConfig
	fixedWidths           tw.Mapper[int, int]
	widthsFinalized       bool
	tableOutputStarted    bool
	headerContentRendered bool // True if actual header *content* has been rendered by Ocean.Header
	logger                *ll.Logger
	w                     io.Writer
}

func NewOcean(oceanConfig ...OceanConfig) *Ocean {
	cfg := defaultOceanRendererConfig()
	oCfg := OceanConfig{}
	if len(oceanConfig) > 0 {
		// Apply user-provided OceanConfig if necessary
	}
	r := &Ocean{
		config:      cfg,
		oceanConfig: oCfg,
		fixedWidths: tw.NewMapper[int, int](),
		logger:      ll.New("ocean"),
	}
	r.resetState()
	return r
}

func (o *Ocean) resetState() {
	o.fixedWidths = tw.NewMapper[int, int]()
	o.widthsFinalized = false
	o.tableOutputStarted = false
	o.headerContentRendered = false
	o.logger.Debug("State reset.")
}

func (o *Ocean) Logger(logger *ll.Logger) {
	o.logger = logger.Namespace("ocean")
}

func (o *Ocean) Config() tw.Rendition {
	return o.config
}

func (o *Ocean) tryFinalizeWidths(ctx tw.Formatting) {
	if o.widthsFinalized {
		return
	}
	if ctx.Row.Widths != nil && ctx.Row.Widths.Len() > 0 {
		o.fixedWidths = ctx.Row.Widths.Clone()
		o.widthsFinalized = true
		o.logger.Debugf("Widths finalized from context: %v", o.fixedWidths)
	} else {
		o.logger.Warn("Attempted to finalize widths, but no width data in context.")
	}
}

func (o *Ocean) Start(w io.Writer) error {
	o.w = w
	o.logger.Debug("Start() called.")
	o.resetState()
	// Top border is drawn by the first component (Header or Row) that has widths
	// OR by an explicit Line() call from table.go's batch renderer.
	return nil
}

// renderTopBorderIfNeeded is called by Header or Row if it's the first to render
// and tableOutputStarted is false.
func (o *Ocean) renderTopBorderIfNeeded(currentPosition tw.Position, ctx tw.Formatting) {
	if !o.tableOutputStarted && o.widthsFinalized {
		// This renderer's config for Top border
		if o.config.Borders.Top.Enabled() && o.config.Settings.Lines.ShowTop.Enabled() {
			o.logger.Debugf("Ocean itself rendering top border (triggered by %s)", currentPosition)
			lineCtx := tw.Formatting{ // Construct specific context for this line
				Row: tw.RowContext{
					Widths:   o.fixedWidths,
					Location: tw.LocationFirst,
					Position: currentPosition,
					Next:     ctx.Row.Current, // The actual first content is "Next" to the top border
				},
				Level: tw.LevelHeader,
			}
			o.Line(lineCtx)
			o.tableOutputStarted = true
		}
	}
}

func (o *Ocean) Header(headers [][]string, ctx tw.Formatting) {
	o.logger.Debugf("Ocean.Header called: IsSubRow=%v, Location=%v, NumLines=%d", ctx.IsSubRow, ctx.Row.Location, len(headers))

	if !o.widthsFinalized {
		o.tryFinalizeWidths(ctx)
	}
	// The batch renderer (table.go/renderHeader) will call Line() for the top border
	// and for the header separator if its main config t.config says so.
	// So, Ocean.Header should *not* draw these itself when in batch mode.
	// For true streaming, table.go's streamRenderHeader would make these Line() calls.

	// Decision: Ocean.Header *only* renders header content.
	// Lines (top border, header separator) are managed by the caller (batch or stream logic in table.go).

	if !o.widthsFinalized {
		o.logger.Error("Ocean.Header: Cannot render content, widths are not finalized.")
		// o.headerContentRendered = true; // No, content wasn't rendered.
		return
	}

	if len(headers) > 0 && len(headers[0]) > 0 {
		for i, headerLineData := range headers {
			currentLineCtx := ctx
			currentLineCtx.Row.Widths = o.fixedWidths
			if i > 0 {
				currentLineCtx.IsSubRow = true
			}
			o.renderContentLine(currentLineCtx, headerLineData)
			o.tableOutputStarted = true // Content was written
		}
		o.headerContentRendered = true
	} else {
		o.logger.Debug("Ocean.Header: No actual header content lines to render.")
		// If header is empty, table.go's renderHeader might still call Line() for the separator.
		// o.headerContentRendered remains false if no content.
	}
	// DO NOT draw the header separator line here. Let table.go's renderHeader or streamRenderHeader call o.Line().
}

func (o *Ocean) Row(row []string, ctx tw.Formatting) {
	o.logger.Debugf("Ocean.Row called: IsSubRow=%v, Location=%v, DataItems=%d", ctx.IsSubRow, ctx.Row.Location, len(row))

	if !o.widthsFinalized {
		o.tryFinalizeWidths(ctx)
	}
	// Top border / header separator logic:
	// If this is the very first output, table.go's batch renderHeader (or streamRenderHeader)
	// should have already called Line() for top border and header separator.
	// If Header() was called but rendered no content, table.go's renderHeader would still call Line() for the separator.
	// If Header() was never called by table.go (e.g. streaming rows directly after Start()),
	// then table.go's streamAppendRow needs to handle initial lines.

	// Decision: Ocean.Row *only* renders row content.

	if !o.widthsFinalized {
		o.logger.Error("Ocean.Row: Cannot render content, widths are not finalized.")
		return
	}

	ctx.Row.Widths = o.fixedWidths
	o.renderContentLine(ctx, row)
	o.tableOutputStarted = true
}

func (o *Ocean) Footer(footers [][]string, ctx tw.Formatting) {
	o.logger.Debugf("Ocean.Footer called: IsSubRow=%v, Location=%v, NumLines=%d", ctx.IsSubRow, ctx.Row.Location, len(footers))

	if !o.widthsFinalized {
		o.tryFinalizeWidths(ctx)
		o.logger.Warn("Ocean.Footer: Widths finalized at Footer stage (unusual).")
	}
	// Separator line before footer:
	// This should be handled by table.go's renderFooter or streamRenderFooter calling o.Line().

	// Decision: Ocean.Footer *only* renders footer content.

	if !o.widthsFinalized {
		o.logger.Error("Ocean.Footer: Cannot render content, widths are not finalized.")
		return
	}

	if len(footers) > 0 && len(footers[0]) > 0 {
		for i, footerLineData := range footers {
			currentLineCtx := ctx
			currentLineCtx.Row.Widths = o.fixedWidths
			if i > 0 {
				currentLineCtx.IsSubRow = true
			}
			o.renderContentLine(currentLineCtx, footerLineData)
			o.tableOutputStarted = true
		}
	} else {
		o.logger.Debug("Ocean.Footer: No actual footer content lines to render.")
	}
	// DO NOT draw the bottom border here. Let table.go's main Close or batch renderFooter call o.Line().
}

func (o *Ocean) Line(ctx tw.Formatting) {
	// This method is now called EXTERNALLY by table.go's batch or stream logic
	// to draw all horizontal lines (top border, header sep, footer sep, bottom border).
	if !o.widthsFinalized {
		// If Line is called before widths are known (e.g. table.go's batch renderHeader trying to draw top border)
		// we must try to finalize widths from this context.
		o.tryFinalizeWidths(ctx)
		if !o.widthsFinalized {
			o.logger.Error("Ocean.Line: Called but widths could not be finalized. Skipping line rendering.")
			return
		}
	}
	// Ensure Line uses the consistent fixedWidths for drawing
	ctx.Row.Widths = o.fixedWidths

	o.logger.Debugf("Ocean.Line DRAWING: Level=%v, Loc=%s, Pos=%s, IsSubRow=%t, WidthsLen=%d", ctx.Level, ctx.Row.Location, ctx.Row.Position, ctx.IsSubRow, ctx.Row.Widths.Len())

	jr := NewJunction(JunctionContext{
		Symbols:       o.config.Symbols,
		Ctx:           ctx,
		ColIdx:        0,
		Logger:        o.logger,
		BorderTint:    Tint{},
		SeparatorTint: Tint{},
	})

	var line strings.Builder
	sortedColIndices := o.fixedWidths.SortedKeys()

	if len(sortedColIndices) == 0 {
		drewEmptyBorders := false
		if o.config.Borders.Left.Enabled() {
			line.WriteString(jr.RenderLeft())
			drewEmptyBorders = true
		}
		if o.config.Borders.Right.Enabled() {
			line.WriteString(jr.RenderRight(-1))
			drewEmptyBorders = true
		}
		if drewEmptyBorders {
			line.WriteString(tw.NewLine)
			o.w.Write([]byte(line.String()))
			o.logger.Debug("Line: Drew empty table borders based on Line call.")
		} else {
			o.logger.Debug("Line: Handled empty table case (no columns, no borders).")
		}
		o.tableOutputStarted = drewEmptyBorders // A line counts as output
		return
	}

	if o.config.Borders.Left.Enabled() {
		line.WriteString(jr.RenderLeft())
	}

	for i, colIdx := range sortedColIndices {
		jr.colIdx = colIdx
		segmentChar := jr.GetSegment()
		colVisualWidth := o.fixedWidths.Get(colIdx)

		if colVisualWidth <= 0 {
			// Still need to consider separators after zero-width columns
		} else {
			if segmentChar == tw.Empty {
				segmentChar = o.config.Symbols.Row()
			}
			segmentDisplayWidth := tw.DisplayWidth(segmentChar)
			if segmentDisplayWidth <= 0 {
				segmentDisplayWidth = 1
			}

			repeatCount := 0
			if colVisualWidth > 0 {
				repeatCount = colVisualWidth / segmentDisplayWidth
				if repeatCount == 0 {
					repeatCount = 1
				}
			}
			line.WriteString(strings.Repeat(segmentChar, repeatCount))
		}

		if i < len(sortedColIndices)-1 && o.config.Settings.Separators.BetweenColumns.Enabled() {
			nextColIdx := sortedColIndices[i+1]
			line.WriteString(jr.RenderJunction(colIdx, nextColIdx))
		}
	}

	if o.config.Borders.Right.Enabled() {
		lastColIdx := sortedColIndices[len(sortedColIndices)-1]
		line.WriteString(jr.RenderRight(lastColIdx))
	}

	line.WriteString(tw.NewLine)
	o.w.Write([]byte(line.String()))
	o.tableOutputStarted = true
	o.logger.Debugf("Line rendered by explicit call: %s", strings.TrimSuffix(line.String(), tw.NewLine))
}

func (o *Ocean) Close() error {
	o.logger.Debug("Ocean.Close() called.")
	// The actual bottom border drawing is expected to be handled by table.go's
	// batch render logic (renderFooter) or stream logic (streamRenderBottomBorder)
	// by making an explicit call to o.Line() with the correct context.
	// Ocean.Close() itself does not draw the bottom border to avoid duplication.

	// Only reset state.
	o.resetState()
	return nil
}

func (o *Ocean) renderContentLine(ctx tw.Formatting, lineData []string) {
	if !o.widthsFinalized || o.fixedWidths.Len() == 0 {
		o.logger.Error("renderContentLine: Cannot render, fixedWidths not set or empty.")
		return
	}

	var output strings.Builder
	if o.config.Borders.Left.Enabled() {
		output.WriteString(o.config.Symbols.Column())
	}

	sortedColIndices := o.fixedWidths.SortedKeys()

	for i, colIdx := range sortedColIndices {
		cellVisualWidth := o.fixedWidths.Get(colIdx)
		cellContent := tw.Empty
		align := tw.AlignDefault
		padding := tw.Padding{Left: tw.Space, Right: tw.Space}

		switch ctx.Row.Position {
		case tw.Header:
			align = tw.AlignCenter
		case tw.Footer:
			align = tw.AlignRight
		default:
			align = tw.AlignLeft
		}

		cellCtx, hasCellCtx := ctx.Row.Current[colIdx]
		if hasCellCtx {
			cellContent = cellCtx.Data
			if cellCtx.Align.Validate() == nil && cellCtx.Align != tw.AlignNone {
				align = cellCtx.Align
			}
			if cellCtx.Padding.Paddable() {
				padding = cellCtx.Padding
			}
		} else if colIdx < len(lineData) {
			cellContent = lineData[colIdx]
		}

		actualCellWidthToRender := cellVisualWidth
		isHMergeContinuation := false

		if hasCellCtx && cellCtx.Merge.Horizontal.Present {
			if cellCtx.Merge.Horizontal.Start {
				hSpan := cellCtx.Merge.Horizontal.Span
				if hSpan <= 0 {
					hSpan = 1
				}

				currentMergeTotalRenderWidth := 0
				for k := 0; k < hSpan; k++ {
					idxInMergeSpan := colIdx + k
					// Check if idxInMergeSpan is a defined column in fixedWidths
					foundInFixedWidths := false
					for _, sortedCIdx_inner := range sortedColIndices {
						if sortedCIdx_inner == idxInMergeSpan {
							currentMergeTotalRenderWidth += o.fixedWidths.Get(idxInMergeSpan)
							foundInFixedWidths = true
							break
						}
					}
					if !foundInFixedWidths && idxInMergeSpan <= sortedColIndices[len(sortedColIndices)-1] {
						o.logger.Debugf("Col %d in HMerge span not found in fixedWidths, assuming 0-width contribution.", idxInMergeSpan)
					}

					if k < hSpan-1 && o.config.Settings.Separators.BetweenColumns.Enabled() {
						currentMergeTotalRenderWidth += tw.DisplayWidth(o.config.Symbols.Column())
					}
				}
				actualCellWidthToRender = currentMergeTotalRenderWidth
			} else {
				isHMergeContinuation = true
			}
		}

		if isHMergeContinuation {
			o.logger.Debugf("renderContentLine: Col %d is HMerge continuation, skipping content render.", colIdx)
			// The separator logic below needs to handle this correctly.
			// If the *previous* column was the start of a merge that spans *this* column,
			// then the separator after the previous column should have been suppressed.
		} else if actualCellWidthToRender > 0 {
			formattedCell := o.formatCellContent(cellContent, actualCellWidthToRender, padding, align)
			output.WriteString(formattedCell)
		} else {
			o.logger.Debugf("renderContentLine: col %d has 0 render width, writing no content.", colIdx)
		}

		// Add column separator if:
		// 1. It's not the last column in sortedColIndices
		// 2. Separators are enabled
		// 3. This cell is NOT a horizontal merge start that spans over the next column.
		if i < len(sortedColIndices)-1 && o.config.Settings.Separators.BetweenColumns.Enabled() {
			shouldAddSeparator := true
			if hasCellCtx && cellCtx.Merge.Horizontal.Present && cellCtx.Merge.Horizontal.Start {
				// If this merge start spans beyond the current colIdx into the next sortedColIndex
				if colIdx+cellCtx.Merge.Horizontal.Span > sortedColIndices[i+1] {
					shouldAddSeparator = false // Separator is part of the merged cell's width
					o.logger.Debugf("renderContentLine: Suppressed separator after HMerge col %d.", colIdx)
				}
			}
			if shouldAddSeparator {
				output.WriteString(o.config.Symbols.Column())
			}
		}
	}

	if o.config.Borders.Right.Enabled() {
		output.WriteString(o.config.Symbols.Column())
	}

	output.WriteString(tw.NewLine)
	o.w.Write([]byte(output.String()))
	o.logger.Debugf("Content line rendered: %s", strings.TrimSuffix(output.String(), tw.NewLine))
}

func (o *Ocean) formatCellContent(content string, cellVisualWidth int, padding tw.Padding, align tw.Align) string {
	if cellVisualWidth <= 0 {
		return tw.Empty
	}

	contentDisplayWidth := tw.DisplayWidth(content)

	padLeftChar := padding.Left
	if padLeftChar == tw.Empty {
		padLeftChar = tw.Space
	}
	padRightChar := padding.Right
	if padRightChar == tw.Empty {
		padRightChar = tw.Space
	}

	padLeftDisplayWidth := tw.DisplayWidth(padLeftChar)
	padRightDisplayWidth := tw.DisplayWidth(padRightChar)

	spaceForContentAndAlignment := cellVisualWidth - padLeftDisplayWidth - padRightDisplayWidth
	if spaceForContentAndAlignment < 0 {
		spaceForContentAndAlignment = 0
	}

	if contentDisplayWidth > spaceForContentAndAlignment {
		content = tw.TruncateString(content, spaceForContentAndAlignment)
		contentDisplayWidth = tw.DisplayWidth(content)
	}

	remainingSpace := spaceForContentAndAlignment - contentDisplayWidth
	if remainingSpace < 0 {
		remainingSpace = 0
	}

	var PL, PR string
	switch align {
	case tw.AlignRight:
		PL = strings.Repeat(tw.Space, remainingSpace)
	case tw.AlignCenter:
		leftSpaces := remainingSpace / 2
		rightSpaces := remainingSpace - leftSpaces
		PL = strings.Repeat(tw.Space, leftSpaces)
		PR = strings.Repeat(tw.Space, rightSpaces)
	default:
		PR = strings.Repeat(tw.Space, remainingSpace)
	}

	var sb strings.Builder
	sb.WriteString(padLeftChar)
	sb.WriteString(PL)
	sb.WriteString(content)
	sb.WriteString(PR)
	sb.WriteString(padRightChar)

	currentFormattedWidth := tw.DisplayWidth(sb.String())
	if currentFormattedWidth < cellVisualWidth {
		if align == tw.AlignRight {
			prefixSpaces := strings.Repeat(tw.Space, cellVisualWidth-currentFormattedWidth)
			finalStr := prefixSpaces + sb.String()
			sb.Reset()
			sb.WriteString(finalStr)
		} else {
			sb.WriteString(strings.Repeat(tw.Space, cellVisualWidth-currentFormattedWidth))
		}
	} else if currentFormattedWidth > cellVisualWidth {
		tempStr := sb.String()
		sb.Reset()
		sb.WriteString(tw.TruncateString(tempStr, cellVisualWidth))
		o.logger.Warnf("formatCellContent: Final string '%s' (width %d) exceeded target %d. Force truncated.", tempStr, currentFormattedWidth, cellVisualWidth)
	}
	return sb.String()
}

func (o *Ocean) Rendition(config tw.Rendition) {
	o.config = mergeRendition(o.config, config)
	o.logger.Debugf("Blueprint.Rendition updated. New internal config: %+v", o.config)
}

// Ensure Blueprint implements tw.Renditioning
var _ tw.Renditioning = (*Ocean)(nil)
