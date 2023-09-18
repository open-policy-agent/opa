package marshal

// JSONOptions defines the options for JSON operations,
// currently only marshaling can be configured
type JSONOptions struct {
	MarshalOptions JSONMarshalOptions
}

// JSONMarshalOptions defines the options for JSON marshaling,
// currently only toggling the marshaling of location information is supported
type JSONMarshalOptions struct {
	IncludeLocation     NodeToggle
	IncludeLocationText bool
}

// NodeToggle is a generic struct to allow the toggling of
// settings for different ast node types
type NodeToggle struct {
	Term           bool
	Package        bool
	Comment        bool
	Import         bool
	Rule           bool
	Head           bool
	Expr           bool
	SomeDecl       bool
	Every          bool
	With           bool
	Annotations    bool
	AnnotationsRef bool
}
