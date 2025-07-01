package tw

// CellFormatting holds formatting options for table cells.
type CellFormatting struct {
	AutoWrap  int // Wrapping behavior (e.g., WrapTruncate, WrapNormal)
	MergeMode int // Bitmask for merge behavior (e.g., MergeHorizontal, MergeVertical)

	// Changed form bool to State
	// See https://github.com/olekukonko/tablewriter/issues/261
	AutoFormat State // Enables automatic formatting (e.g., title case for headers)

	// Deprecated: kept for compatibility
	// will be removed soon
	Alignment Align // Text alignment within the cell (e.g., Left, Right, Center)

}

// CellPadding defines padding settings for table cells.
type CellPadding struct {
	Global    Padding   // Default padding applied to all cells
	PerColumn []Padding // Column-specific padding overrides
}

// CellFilter defines filtering functions for cell content.
type CellFilter struct {
	Global    func([]string) []string // Processes the entire row
	PerColumn []func(string) string   // Processes individual cells by column
}

// CellCallbacks holds callback functions for cell processing.
// Note: These are currently placeholders and not fully implemented.
type CellCallbacks struct {
	Global    func()   // Global callback applied to all cells
	PerColumn []func() // Column-specific callbacks
}

// CellAlignment defines alignment settings for table cells.
type CellAlignment struct {
	Global    Align   // Default alignment applied to all cells
	PerColumn []Align // Column-specific alignment overrides
}

// CellConfig combines formatting, padding, and callback settings for a table section.
type CellConfig struct {
	Formatting   CellFormatting // Cell formatting options
	Padding      CellPadding    // Padding configuration
	Callbacks    CellCallbacks  // Callback functions (unused)
	Filter       CellFilter     // Function to filter cell content (renamed from Filter Filter)
	Alignment    CellAlignment  // Alignment configuration for cells
	ColMaxWidths CellWidth      // Per-column maximum width overrides

	// Deprecated: use Alignment.PerColumn instead. Will be removed in a future version.
	// will be removed soon
	ColumnAligns []Align // Per-column alignment overrides
}

type CellWidth struct {
	Global    int
	PerColumn Mapper[int, int]
}

func (c CellWidth) Constrained() bool {
	return c.Global > 0 || c.PerColumn.Len() > 0
}
