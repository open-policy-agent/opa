// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package repl_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/repl"
	"github.com/open-policy-agent/opa/storage/inmem"
)

// nolint // example code
func ExampleREPL_OneShot() {
	// Initialize context for the example. Normally the caller would obtain the
	// context from an input parameter or instantiate their own.
	ctx := context.Background()

	// Instantiate the policy engine's storage layer.
	store := inmem.New()

	// Create a buffer that will receive REPL output.
	var buf bytes.Buffer

	// Create a new REPL.
	r := repl.New(store, "", &buf, "json", 0, "")

	// Define a rule inside the REPL.
	r.OneShot(ctx, "p { a = [1, 2, 3, 4]; a[_] > 3 }")

	// Query the rule defined above.
	r.OneShot(ctx, "p")

	// Inspect the output. Defining rules does not produce output so we only expect
	// output from the second line of input.
	fmt.Println(buf.String())

	// Output:
	// {
	//   "result": [
	//     {
	//       "expressions": [
	//         {
	//           "value": true,
	//           "text": "p",
	//           "location": {
	//             "row": 1,
	//             "col": 1
	//           }
	//         }
	//       ]
	//     }
	//   ]
	// }
}
