#!/usr/bin/env bash
# Regenerate internal/methodlesstemplate from the Go stdlib text/template.
#
# internal/methodlesstemplate is a verbatim copy of the standard library's
# text/template with exactly ONE behavioral edit: the method-calls-on-data
# branch in exec.go's evalField (reflect.Value.MethodByName) is removed. A
# reachable non-constant MethodByName forces the Go linker to disable
# method-level dead-code elimination for the whole binary (golang/go#72895), so
# eliding it is what lets OPA embedders shed the unused reflected method surface
# (#7903). Rego values and gojsonschema ErrorDetails decode to
# map[string]any/[]any/scalars, which have no methods, so the elision is a no-op.
#
# This script re-syncs that copy to whatever Go toolchain `go` resolves. Bump the
# vendored version by running it under a newer toolchain, e.g.:
#   GOTOOLCHAIN=go1.26.0 build/regen-methodless-template.sh
#
# It copies the stdlib files verbatim, then re-applies the two-hunk exec.go edit
# (below) via `git apply`. helper.go and *_test.go are intentionally NOT vendored
# (OPA only needs New/Parse/Execute), and text/template/parse is reused from the
# stdlib via its normal import, so it is not copied here.
#
# If `git apply` fails, the stdlib changed the import block or the evalField
# region this edit targets: re-derive the elision by hand and update the PATCH
# heredoc at the bottom of this script.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

if [[ "$(go list -m 2>/dev/null)" != "github.com/open-policy-agent/opa" ]]; then
	echo "error: must run inside the open-policy-agent/opa module" >&2
	exit 1
fi

GOROOT="$(go env GOROOT)"
TMPL_SRC="$GOROOT/src/text/template"
PKG="internal/methodlesstemplate"

echo "Regenerating $PKG from $(go version)"

# Verbatim stdlib copies. exec.go is patched below; the rest are byte-identical.
mkdir -p "$PKG/internal/fmtsort"
for f in doc.go exec.go funcs.go option.go template.go; do
	cp -f "$TMPL_SRC/$f" "$PKG/$f"
done
cp -f "$GOROOT/src/internal/fmtsort/sort.go" "$PKG/internal/fmtsort/sort.go"
cp -f "$GOROOT/LICENSE" "$PKG/LICENSE"

# Re-apply the single intended edit to exec.go: swap the internal/fmtsort import
# for the vendored one, and remove the MethodByName data-method branch.
# --recount tolerates line-number drift from unrelated stdlib changes; it fails
# only if the patched context text itself changed.
if ! git apply --recount <<'PATCH'
--- a/internal/methodlesstemplate/exec.go
+++ b/internal/methodlesstemplate/exec.go
@@ -7,12 +7,13 @@
 import (
 	"errors"
 	"fmt"
-	"internal/fmtsort"
 	"io"
 	"reflect"
 	"runtime"
 	"strings"
 	"text/template/parse"
+
+	"github.com/open-policy-agent/opa/internal/methodlesstemplate/internal/fmtsort"
 )

 // maxExecDepth specifies the maximum stack depth of templates within
@@ -689,21 +690,18 @@
 	typ := receiver.Type()
 	receiver, isNil := indirect(receiver)
 	if receiver.Kind() == reflect.Interface && isNil {
-		// Calling a method on a nil interface can't work. The
-		// MethodByName method call below would panic.
+		// Indexing into a nil interface can't work.
 		s.errorf("nil pointer evaluating %s.%s", typ, fieldName)
 		return zero
 	}

-	// Unless it's an interface, need to get to a value of type *T to guarantee
-	// we see all methods of T and *T.
-	ptr := receiver
-	if ptr.Kind() != reflect.Interface && ptr.Kind() != reflect.Pointer && ptr.CanAddr() {
-		ptr = ptr.Addr()
-	}
-	if method := ptr.MethodByName(fieldName); method.IsValid() {
-		return s.evalCall(dot, method, false, node, fieldName, args, final)
-	}
+	// OPA-DCE (#7903): the upstream text/template resolves methods on the data
+	// value here via reflect.Value.MethodByName. A reachable non-constant
+	// MethodByName disables the Go linker's method-level dead-code elimination
+	// binary-wide (golang/go#72895), so that branch is deliberately removed.
+	// Rego values (and gojsonschema ErrorDetails) decode to
+	// map[string]any/[]any/scalars, which have no methods, so field/element
+	// resolution below is the only path OPA's callers need.
 	hasArgs := len(args) > 1 || !isMissing(final)
 	// It's not a method; must be a field of a struct or an element of a map.
 	switch receiver.Kind() {
PATCH
then
	echo "error: could not apply the method-elision patch to exec.go." >&2
	echo "       The stdlib import block or evalField region changed; re-derive" >&2
	echo "       the edit by hand and update the PATCH heredoc in $0." >&2
	exit 1
fi

gofmt -w "$PKG"

# Fidelity guard: the whole point is that no reflect method-call on data survives.
if grep -rn '\.MethodByName(' "$PKG"; then
	echo "error: $PKG still contains a .MethodByName( call after patching." >&2
	exit 1
fi

go build ./"$PKG"/...

echo "Done. Vendored $PKG from $(go version)."
echo "Review 'git diff $PKG' before committing; run 'make go-test' for full verification."
