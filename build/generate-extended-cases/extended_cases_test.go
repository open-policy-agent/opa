package cases

import "testing"

func TestLoadExtended(t *testing.T) {
	// If a test case fails to create an IR plan an error will be returned
	// Seems unnecessary to check each individual test if the plan was generated correctly
	_, err := LoadIrExtendedTestCases()
	if err != nil {
		t.Fatal(err)
	}
}
