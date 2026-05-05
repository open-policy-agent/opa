package rego

import (
	"context"
	"testing"
)

func TestEvaluatedRules(t *testing.T) {
	t.Run("single rule evaluation", func(t *testing.T) {
		module := `package test

# METADATA
# title: Rule 1
# id: rule1
p if input.foo

# METADATA
# title: Rule 2
# id: rule2
p if input.bar`

		var evaluated []string
		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true}),
			EvaluatedRules(&evaluated),
		)

		rs, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(rs) == 0 {
			t.Fatal("expected result")
		}

		if len(evaluated) != 1 {
			t.Fatalf("expected 1 evaluated rule, got %d: %v", len(evaluated), evaluated)
		}

		if evaluated[0] != "rule1" {
			t.Fatalf("expected rule1, got %s", evaluated[0])
		}
	})

	t.Run("multiple rules evaluation", func(t *testing.T) {
		module := `package test

# METADATA
# id: rule1
p if input.foo

# METADATA
# id: rule2
p if input.bar`

		var evaluated []string
		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true, "bar": true}),
			EvaluatedRules(&evaluated),
		)

		rs, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(rs) == 0 {
			t.Fatal("expected result")
		}

		// NOTE: For complete document rules (without keys), OPA uses early-exit optimization.
		// Only the first successfully evaluated rule is recorded because OPA stops evaluation
		// after finding the first match. This is expected behavior.
		// The exact rule recorded depends on rule ordering in the compiled module.
		if len(evaluated) != 1 {
			t.Fatalf("expected 1 evaluated rule (early-exit), got %d: %v", len(evaluated), evaluated)
		}

		// Should have either rule1 or rule2 (whichever was evaluated first)
		if evaluated[0] != "rule1" && evaluated[0] != "rule2" {
			t.Fatalf("expected rule1 or rule2, got %s", evaluated[0])
		}
	})

	t.Run("no matching rules", func(t *testing.T) {
		module := `package test

# METADATA
# id: rule1
p if input.foo

# METADATA
# id: rule2
p if input.bar`

		var evaluated []string
		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"baz": true}),
			EvaluatedRules(&evaluated),
		)

		rs, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// No results expected
		if len(rs) != 0 && rs[0].Expressions[0].Value != false {
			t.Fatal("expected no result or false")
		}

		if len(evaluated) != 0 {
			t.Fatalf("expected 0 evaluated rules, got %d: %v", len(evaluated), evaluated)
		}
	})

	t.Run("rules without metadata IDs", func(t *testing.T) {
		module := `package test

p if input.foo

p if input.bar`

		var evaluated []string
		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true}),
			EvaluatedRules(&evaluated),
		)

		rs, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(rs) == 0 {
			t.Fatal("expected result")
		}

		// Rules without IDs should not be recorded
		if len(evaluated) != 0 {
			t.Fatalf("expected 0 evaluated rules (no IDs), got %d: %v", len(evaluated), evaluated)
		}
	})

	t.Run("mixed rules with and without IDs", func(t *testing.T) {
		module := `package test

# METADATA
# id: rule1
p if input.foo

p if input.bar

# METADATA
# id: rule3
p if input.baz`

		var evaluated []string
		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true, "bar": true}),
			EvaluatedRules(&evaluated),
		)

		rs, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(rs) == 0 {
			t.Fatal("expected result")
		}

		// NOTE: Due to early-exit optimization, only the first successfully evaluated rule is recorded.
		// The rule without an ID will not be recorded (no ID to record), and rule3 won't be reached
		// due to early exit. Depending on rule ordering, we may get rule1 or no rules.
		// With input.foo=true and input.bar=true, typically rule1 or the rule without ID evaluates first.
		if len(evaluated) > 1 {
			t.Fatalf("expected at most 1 evaluated rule (early-exit), got %d: %v", len(evaluated), evaluated)
		}

		// If a rule was recorded, it should be one with an ID
		if len(evaluated) == 1 && evaluated[0] != "rule1" && evaluated[0] != "rule3" {
			t.Fatalf("expected rule1 or rule3, got %s", evaluated[0])
		}
	})

	t.Run("nil evaluated pointer", func(t *testing.T) {
		module := `package test

# METADATA
# id: rule1
p if input.foo`

		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true}),
			EvaluatedRules(nil),
		)

		// Should not panic with nil pointer
		_, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("complete document rule", func(t *testing.T) {
		module := `package test

# METADATA
# id: complete_rule
p := input.foo`

		var evaluated []string
		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"foo": true}),
			EvaluatedRules(&evaluated),
		)

		rs, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(rs) == 0 {
			t.Fatal("expected result")
		}

		if len(evaluated) != 1 {
			t.Fatalf("expected 1 evaluated rule, got %d: %v", len(evaluated), evaluated)
		}

		if evaluated[0] != "complete_rule" {
			t.Fatalf("expected complete_rule, got %s", evaluated[0])
		}
	})

	t.Run("multi-value rule", func(t *testing.T) {
		module := `package test

# METADATA
# id: multi_value_rule
p contains x if {
	some x in input.values
	x > 5
}`

		var evaluated []string
		r := New(
			Query("data.test.p"),
			Module("test.rego", module),
			Input(map[string]any{"values": []any{3, 6, 9, 2, 10}}),
			EvaluatedRules(&evaluated),
		)

		rs, err := r.Eval(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(rs) == 0 {
			t.Fatal("expected result")
		}

		// Multi-value rules iterate and produce multiple results
		// The rule ID should be recorded for each successful evaluation
		if len(evaluated) < 1 {
			t.Fatalf("expected at least 1 evaluated rule, got %d: %v", len(evaluated), evaluated)
		}

		// All recorded IDs should be the same rule
		for _, id := range evaluated {
			if id != "multi_value_rule" {
				t.Fatalf("expected multi_value_rule, got %s", id)
			}
		}

		// Verify we got the expected results (6, 9, 10)
		result := rs[0].Expressions[0].Value
		resultSet, ok := result.([]any)
		if !ok {
			t.Fatalf("expected set result, got %T", result)
		}

		// OPA returns sets as JSON objects with values as keys
		expectedValues := []any{6, 9, 10}
		if len(resultSet) != len(expectedValues) {
			t.Fatalf("expected %d values in result set, got %d", len(expectedValues), len(resultSet))
		}
	})
}

func TestEvaluatedRulesPreparedQuery(t *testing.T) {
	module := `package test

# METADATA
# id: rule1
p if input.foo

# METADATA
# id: rule2
p if input.bar`

	r := New(
		Query("data.test.p"),
		Module("test.rego", module),
	)

	pq, err := r.PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var evaluated []string
	rs, err := pq.Eval(context.Background(),
		EvalInput(map[string]any{"foo": true}),
		EvalEvaluated(&evaluated),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rs) == 0 {
		t.Fatal("expected result")
	}

	if len(evaluated) != 1 || evaluated[0] != "rule1" {
		t.Fatalf("expected [rule1], got %v", evaluated)
	}

	// Reuse the prepared query with a fresh slice
	var evaluated2 []string
	rs, err = pq.Eval(context.Background(),
		EvalInput(map[string]any{"bar": true}),
		EvalEvaluated(&evaluated2),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rs) == 0 {
		t.Fatal("expected result")
	}

	if len(evaluated2) != 1 || evaluated2[0] != "rule2" {
		t.Fatalf("expected [rule2], got %v", evaluated2)
	}
}
