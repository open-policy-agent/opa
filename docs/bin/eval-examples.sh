#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

failed=0
passed=0
skipped=0
total=0

while IFS= read -r config_file; do
  dir="$(dirname "$config_file")"
  total=$((total + 1))
  rel_dir="${dir#$ROOT_DIR/}"

  # Check if this example should be skipped
  skip_reason=$(jq -r '.skip_output_reason // empty' "$config_file")
  if [[ -n "$skip_reason" ]]; then
    echo "SKIP: $rel_dir ($skip_reason)"
    skipped=$((skipped + 1))
    continue
  fi

  # Read the command from config.json, default to data.play
  command=$(jq -r '.command // "data.play"' "$config_file")

  # Build the opa eval args
  args=()
  args+=(-d "$dir/policy.rego")

  if [[ -f "$dir/input.json" ]]; then
    args+=(-i "$dir/input.json")
  fi

  if [[ -f "$dir/data.json" ]]; then
    args+=(-d "$dir/data.json")
  fi

  args+=("$command")
  args+=(-f pretty)

  # Run opa eval and write output
  if output=$(opa eval "${args[@]}" 2>&1); then
    echo "$output" > "$dir/output.json"
    echo "PASS: $rel_dir"
    passed=$((passed + 1))
  else
    echo "FAIL: $rel_dir"
    echo "  $output" | head -5
    failed=$((failed + 1))
  fi
done < <(find "$ROOT_DIR" -name config.json -path '*/_examples/*')

echo ""
echo "Results: $passed passed, $failed failed, $skipped skipped, $total total"
exit $((failed > 0 ? 1 : 0))
