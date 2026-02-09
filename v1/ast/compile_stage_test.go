// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"
)

func TestCompilerStageSkipping(t *testing.T) {
	tests := []struct {
		name             string
		skipStages       []StageID
		expectedSkips    []string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:       "no stages skipped",
			skipStages: nil,
			shouldContain: []string{
				"ResolveRefs",
				"CheckTypes",
				"BuildRuleIndices",
			},
		},
		{
			name: "skip type checking",
			skipStages: []StageID{
				StageCheckTypes,
			},
			shouldContain: []string{
				"ResolveRefs",
				"BuildRuleIndices",
			},
			shouldNotContain: []string{
				"CheckTypes",
			},
		},
		{
			name: "skip multiple stages",
			skipStages: []StageID{
				StageBuildRuleIndices,
				StageBuildComprehensionIndices,
			},
			shouldContain: []string{
				"ResolveRefs",
				"CheckTypes",
			},
			shouldNotContain: []string{
				"BuildRuleIndices",
				"BuildComprehensionIndices",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCompiler()
			if len(tt.skipStages) > 0 {
				c.WithSkipStages(tt.skipStages...)
			}

			// Trigger initialization
			c.Compile(map[string]*Module{})

			stages := c.StagesToRun()
			stageNames := make(map[string]bool)
			for _, s := range stages {
				stageNames[string(s)] = true
			}

			// Check stages that should be present
			for _, name := range tt.shouldContain {
				if !stageNames[name] {
					t.Errorf("expected stage %q to be present, but it was not", name)
				}
			}

			// Check stages that should NOT be present
			for _, name := range tt.shouldNotContain {
				if stageNames[name] {
					t.Errorf("expected stage %q to be skipped, but it was present", name)
				}
			}
		})
	}
}

func TestCompilerStageSkippingWithEvalMode(t *testing.T) {
	t.Run("EvalModeIR skips index building stages", func(t *testing.T) {
		c := NewCompiler().WithEvalMode(EvalModeIR)
		c.Compile(map[string]*Module{})

		stages := c.StagesToRun()
		stageNames := make(map[string]bool)
		for _, s := range stages {
			stageNames[string(s)] = true
		}

		// These should be skipped in IR mode
		if stageNames["BuildRuleIndices"] {
			t.Error("BuildRuleIndices should be skipped in EvalModeIR")
		}
		if stageNames["BuildComprehensionIndices"] {
			t.Error("BuildComprehensionIndices should be skipped in EvalModeIR")
		}

		// Other stages should still be present
		if !stageNames["ResolveRefs"] {
			t.Error("ResolveRefs should be present")
		}
	})

	t.Run("additional skip stages with EvalModeIR", func(t *testing.T) {
		c := NewCompiler().
			WithEvalMode(EvalModeIR).
			WithSkipStages(StageCheckTypes)
		c.Compile(map[string]*Module{})

		stages := c.StagesToRun()
		stageNames := make(map[string]bool)
		for _, s := range stages {
			stageNames[string(s)] = true
		}

		// Both IR mode skips and explicit skips should be applied
		if stageNames["BuildRuleIndices"] {
			t.Error("BuildRuleIndices should be skipped in EvalModeIR")
		}
		if stageNames["CheckTypes"] {
			t.Error("CheckTypes should be explicitly skipped")
		}
	})
}

func TestAllStages(t *testing.T) {
	allStages := AllStages()

	if len(allStages) == 0 {
		t.Fatal("AllStages() should return non-empty list")
	}

	// Check some expected stages are present
	expectedStages := []StageID{
		StageResolveRefs,
		StageCheckTypes,
		StageBuildRuleIndices,
		StageBuildComprehensionIndices,
	}

	stageMap := make(map[StageID]bool)
	for _, s := range allStages {
		stageMap[s] = true
	}

	for _, expected := range expectedStages {
		if !stageMap[expected] {
			t.Errorf("expected stage %q to be in AllStages(), but it was not", expected)
		}
	}

	// Verify AllStages matches what a compiler would run
	c := NewCompiler()
	c.Compile(map[string]*Module{})

	planned := c.StagesToRun()
	if len(allStages) != len(planned) {
		t.Errorf("AllStages() returned %d stages but compiler plans to run %d stages", len(allStages), len(planned))
	}

	// Check that AllStages and planned stages match
	for i, expected := range allStages {
		if i >= len(planned) {
			break
		}
		if expected != planned[i] {
			t.Errorf("AllStages()[%d] = %q, but compiler plans to run %q", i, expected, planned[i])
		}
	}
}

func TestWithOnlyStagesUpToInternal(t *testing.T) {
	tests := []struct {
		name             string
		target           StageID
		expectedCount    int
		shouldContain    []StageID
		shouldNotContain []StageID
	}{
		{
			name:          "up to SetRuleTree",
			target:        StageSetRuleTree,
			expectedCount: 8,
			shouldContain: []StageID{
				StageResolveRefs,
				StageSetModuleTree,
				StageSetRuleTree,
			},
			shouldNotContain: []StageID{
				StageRewriteLocalVars,
				StageBuildRuleIndices,
			},
		},
		{
			name:          "up to BuildRuleIndices",
			target:        StageBuildRuleIndices,
			expectedCount: 32, // includes "after" stage from init()
			shouldContain: []StageID{
				StageResolveRefs,
				StageCheckTypes,
				StageBuildRuleIndices,
			},
			shouldNotContain: []StageID{
				StageBuildComprehensionIndices,
				StageBuildRequiredCapabilities,
			},
		},
		{
			name:          "up to last stage",
			target:        StageBuildRequiredCapabilities,
			expectedCount: 34, // includes "after" stage from init()
			shouldContain: []StageID{
				StageResolveRefs,
				StageBuildRequiredCapabilities,
			},
			shouldNotContain: []StageID{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCompiler().WithOnlyStagesUpTo(tc.target)
			stages := c.StagesToRun()

			if len(stages) != tc.expectedCount {
				t.Errorf("expected %d stages, got %d", tc.expectedCount, len(stages))
			}

			stageMap := make(map[StageID]bool)
			for _, s := range stages {
				stageMap[s] = true
			}

			for _, expected := range tc.shouldContain {
				if !stageMap[expected] {
					t.Errorf("expected stage %q to be present", expected)
				}
			}

			for _, notExpected := range tc.shouldNotContain {
				if stageMap[notExpected] {
					t.Errorf("expected stage %q to NOT be present", notExpected)
				}
			}
		})
	}
}

func TestCompilerStageSkippingWithAfterStages(t *testing.T) {
	t.Run("after stages are included in plan", func(t *testing.T) {
		c := NewCompiler()

		called := false
		c.WithStageAfter("CheckTypes", CompilerStageDefinition{
			Name:       "CustomAfterCheckTypes",
			MetricName: "custom_after_check_types",
			Stage: func(c *Compiler) *Error {
				called = true
				return nil
			},
		})

		c.Compile(map[string]*Module{})

		stages := c.StagesToRun()
		found := false
		for _, s := range stages {
			if s == "CustomAfterCheckTypes" {
				found = true
				break
			}
		}

		if !found {
			t.Error("after stage should be in StagesToRun()")
		}

		if !called {
			t.Error("after stage should have been executed")
		}
	})

	t.Run("skipped main stage skips after stages", func(t *testing.T) {
		c := NewCompiler().WithSkipStages(StageCheckTypes)

		called := false
		c.WithStageAfter("CheckTypes", CompilerStageDefinition{
			Name:       "CustomAfterCheckTypes",
			MetricName: "custom_after_check_types",
			Stage: func(c *Compiler) *Error {
				called = true
				return nil
			},
		})

		c.Compile(map[string]*Module{})

		stages := c.StagesToRun()
		for _, s := range stages {
			if s == "CustomAfterCheckTypes" {
				t.Error("after stage should not be in plan when main stage is skipped")
			}
		}

		if called {
			t.Error("after stage should not have been executed when main stage is skipped")
		}
	})

	t.Run("after stage can be individually skipped", func(t *testing.T) {
		c := NewCompiler()

		called := false
		c.WithStageAfter("CheckTypes", CompilerStageDefinition{
			Name:       "CustomAfterCheckTypes",
			MetricName: "custom_after_check_types",
			Stage: func(c *Compiler) *Error {
				called = true
				return nil
			},
		})
		c.WithSkipStages("CustomAfterCheckTypes")

		c.Compile(map[string]*Module{})

		stages := c.StagesToRun()

		// Main stage should still be present
		hasMainStage := false
		for _, s := range stages {
			if s == StageCheckTypes {
				hasMainStage = true
			}
			if s == "CustomAfterCheckTypes" {
				t.Error("after stage should be skipped")
			}
		}

		if !hasMainStage {
			t.Error("main stage should still be present")
		}

		if called {
			t.Error("after stage should not have been executed")
		}
	})
}
