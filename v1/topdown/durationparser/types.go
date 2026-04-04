package durationparser

// Result holds the parsed components of a duration string.
type Result struct {
	Sign     string // "" or "-" or "+"
	Segments []Segment
}

// Segment holds a single parsed segment (e.g. Digits="1.5", Unit="d").
type Segment struct {
	Digits string
	Unit   string
}
