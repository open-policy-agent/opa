package testdata

import "embed"

// V1 is used by the function `LoadIrExtendedTestCases` under build/generate-extended-cases
// this function adds the IR plan to each test case and is used by OPA IR languages (such as opa-swift)
//
//go:embed v1
var V1 embed.FS
