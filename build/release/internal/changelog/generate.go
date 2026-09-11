package changelog

// Options configures Generate.
type Options struct {
	RepoURL string
	// IncludeLocal keeps entries whose commits are not on the remote.
	IncludeLocal bool
	// FromGoMod and ToGoMod drive dependency synthesis when both are set.
	FromGoMod string
	ToGoMod   string
}

// Result is the rendered body plus the decision trail the CLI prints for review.
type Result struct {
	// Body carries no "## <version>" heading; see Section and Splice.
	Body         string
	DroppedLocal []Entry
	// Ambiguities is the review checklist, empty when nothing needs attention.
	Ambiguities string

	Transforms []TransformLog
	Filters    []FilterLog
	Syntheses  []SynthesisLog
}

// Generate runs the pure part of the pipeline, so the CLI and the golden tests
// share one code path.
func Generate(entries []Entry, opts Options) Result {
	var res Result

	if !opts.IncludeLocal {
		entries, res.DroppedLocal = DropLocalOnly(entries)
	}

	res.Transforms = Transform(entries)

	entries, res.Filters = FilterReleaseMechanics(entries)
	entries, depFilters := FilterDependencies(entries)
	res.Filters = append(res.Filters, depFilters...)

	if opts.FromGoMod != "" && opts.ToGoMod != "" {
		changes := DiffRequires(opts.FromGoMod, opts.ToGoMod)
		entries, res.Syntheses = SynthesizeMissingDeps(entries, changes)
	}

	res.Body = Render(entries, opts.RepoURL)
	res.Ambiguities = AmbiguitiesReport(entries, res.DroppedLocal)
	return res
}
