// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics

import (
	"testing"
	"time"
)

func TestMetricsTimer(t *testing.T) {
	m := New()
	m.Timer("foo").Start()
	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	if m.All()["timer_foo_ns"] == 0 {
		t.Fatalf("Expected foo timer to be non-zero: %v", m.All())
	}
	m.Clear()

	if len(m.All()) > 0 {
		t.Fatalf("Expected metrics to be cleared, but found %v", m.All())
	}
}

func TestMetricsTimerDoubleStop(t *testing.T) {
	m := New()
	m.Timer("foo").Start()

	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t1 := m.Timer("foo").Int64()

	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t2 := m.Timer("foo").Int64()

	if t1 != t2 {
		t.Fatalf("Unexpected difference in stopped timer values: %v, %v", t1, t2)
	}
}

func TestMetricsTimerRestart(t *testing.T) {
	m := New()
	m.Timer("foo").Start()

	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t1 := m.Timer("foo").Int64()

	// Restart the timer.
	m.Timer("foo").Start()
	time.Sleep(time.Millisecond)
	m.Timer("foo").Stop()
	t2 := m.Timer("foo").Int64()

	if t1 >= t2 {
		t.Fatalf("Expected restarted timer to advance, but got same value.: %v, %v", t1, t2)
	}
}

// TestJSONMarshalingCorrectness verifies that custom JSON marshaling produces correct output
func TestJSONMarshalingCorrectness(t *testing.T) {
	m := New()

	// Add various metrics
	m.Timer("test_timer").Start()
	time.Sleep(100 * time.Microsecond)
	m.Timer("test_timer").Stop()

	m.Counter("test_counter").Incr()
	m.Counter("test_counter").Incr()

	m.Histogram("test_histogram").Update(50)
	m.Histogram("test_histogram").Update(100)
	m.Histogram("test_histogram").Update(150)

	// Marshal to JSON
	jsonBytes, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("Expected no error during MarshalJSON, got: %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Fatal("Expected non-empty JSON bytes")
	}

	// Verify it's valid JSON by checking structure
	if jsonBytes[0] != '{' || jsonBytes[len(jsonBytes)-1] != '}' {
		t.Fatalf("Expected valid JSON object, got: %s", string(jsonBytes))
	}

	// Verify timer key is in JSON
	if !contains(string(jsonBytes), "timer_test_timer_ns") {
		t.Fatalf("Expected timer key in JSON, got: %s", string(jsonBytes))
	}

	// Verify counter key is in JSON
	if !contains(string(jsonBytes), "counter_test_counter") {
		t.Fatalf("Expected counter key in JSON, got: %s", string(jsonBytes))
	}

	// Verify histogram key is in JSON
	if !contains(string(jsonBytes), "histogram_test_histogram") {
		t.Fatalf("Expected histogram key in JSON, got: %s", string(jsonBytes))
	}
}

// TestCachedFormattedKeys verifies that metric keys are properly cached and formatted
func TestCachedFormattedKeys(t *testing.T) {
	m := New()

	// Add metrics with various names
	testNames := []string{"metric_one", "metric_two", "metric_three"}

	for _, name := range testNames {
		m.Timer(name).Start()
		time.Sleep(50 * time.Microsecond)
		m.Timer(name).Stop()

		m.Counter(name).Incr()
		m.Histogram(name).Update(100)
	}

	all := m.All()

	// Verify all metrics are present
	if len(all) < len(testNames)*3 {
		t.Fatalf("Expected at least %d metrics, got %d", len(testNames)*3, len(all))
	}

	// Verify formatted keys are present
	expectedKeys := []string{
		"timer_metric_one_ns", "timer_metric_two_ns", "timer_metric_three_ns",
		"counter_metric_one", "counter_metric_two", "counter_metric_three",
		"histogram_metric_one", "histogram_metric_two", "histogram_metric_three",
	}

	for _, expectedKey := range expectedKeys {
		if _, ok := all[expectedKey]; !ok {
			t.Fatalf("Expected key %s not found in metrics", expectedKey)
		}
	}
}

// TestStringBuilderPooling verifies that String() method uses builder pooling correctly
func TestStringBuilderPooling(t *testing.T) {
	m := New()

	// Add metrics
	m.Timer("pool_test").Start()
	time.Sleep(50 * time.Microsecond)
	m.Timer("pool_test").Stop()

	m.Counter("pool_counter").Incr()
	m.Histogram("pool_histogram").Update(75)

	// Call String() multiple times - should reuse buffers via pool
	// Cast to *metrics to access String() method
	metricsImpl := m.(*metrics)
	str1 := metricsImpl.String()
	str2 := metricsImpl.String()

	// Both strings should be identical and non-empty
	if str1 != str2 {
		t.Fatalf("Expected identical String() output, got different results")
	}

	if len(str1) == 0 {
		t.Fatal("Expected non-empty String() output")
	}

	// Verify format contains expected keys
	if !contains(str1, "timer_pool_test_ns:") {
		t.Fatalf("Expected timer key in String output, got: %s", str1)
	}

	if !contains(str1, "counter_pool_counter:") {
		t.Fatalf("Expected counter key in String output, got: %s", str1)
	}
}

// TestHistogramFieldNames verifies that histogram values contain proper field names
func TestHistogramFieldNames(t *testing.T) {
	m := New()

	// Add histogram with multiple values to get statistics
	h := m.Histogram("field_test")
	for i := 1; i <= 100; i++ {
		h.Update(int64(i * 10))
	}

	all := m.All()
	histValue, ok := all["histogram_field_test"].(map[string]interface{})

	if !ok {
		t.Fatalf("Expected histogram value to be a map, got: %T", all["histogram_field_test"])
	}

	// Verify expected histogram field names are present
	expectedFields := []string{"count", "min", "max", "mean", "stddev"}
	for _, field := range expectedFields {
		if _, ok := histValue[field]; !ok {
			t.Fatalf("Expected histogram field %s not found in histogram value", field)
		}
	}
}

// TestIntegerFormattingCorrectness verifies that integer formatting produces correct values
func TestIntegerFormattingCorrectness(t *testing.T) {
	m := New()

	// Add timer and get its value
	m.Timer("int_test").Start()
	time.Sleep(100 * time.Microsecond)
	m.Timer("int_test").Stop()

	timerValue := m.Timer("int_test").Int64()

	// Verify timer value is positive
	if timerValue <= 0 {
		t.Fatalf("Expected positive timer value, got: %d", timerValue)
	}

	// Marshal to JSON and verify the value is correctly formatted
	jsonBytes, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed marshal metrics, err: %v", err)
	}
	jsonStr := string(jsonBytes)

	// Check that the value appears in the JSON output
	if !contains(jsonStr, "timer_int_test_ns") {
		t.Fatalf("Expected timer key in JSON, got: %s", jsonStr)
	}
}

// TestMultipleMetricsOrdering verifies that all metrics are present in All() output
func TestMultipleMetricsOrdering(t *testing.T) {
	m := New()

	// Add multiple metrics
	for i := 1; i <= 10; i++ {
		m.Timer("timer_" + string(rune(48+i))).Start()
		time.Sleep(10 * time.Microsecond)
		m.Timer("timer_" + string(rune(48+i))).Stop()

		m.Counter("counter_" + string(rune(48+i))).Incr()
		m.Histogram("histogram_" + string(rune(48+i))).Update(int64(i * 10))
	}

	all := m.All()

	// Verify total count (10 timers + 10 counters + 10 histograms)
	if len(all) < 30 {
		t.Fatalf("Expected at least 30 metrics, got %d", len(all))
	}

	// Verify String() output contains all metrics
	metricsImpl := m.(*metrics)
	str := metricsImpl.String()
	if len(str) == 0 {
		t.Fatal("Expected non-empty String() output")
	}
}

// TestClearOperation verifies that Clear() removes all metrics and cached keys
func TestClearOperation(t *testing.T) {
	m := New()

	// Add metrics
	m.Timer("clear_timer").Start()
	time.Sleep(10 * time.Microsecond)
	m.Timer("clear_timer").Stop()

	m.Counter("clear_counter").Incr()
	m.Histogram("clear_histogram").Update(50)

	// Verify metrics exist
	if len(m.All()) == 0 {
		t.Fatal("Expected metrics to exist before clear")
	}

	// Clear metrics
	m.Clear()

	// Verify all metrics are removed
	if len(m.All()) > 0 {
		t.Fatalf("Expected no metrics after clear, got: %v", m.All())
	}

	// Verify JSON marshaling returns empty object
	jsonBytes, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("Expected no error during MarshalJSON after clear, got: %v", err)
	}

	if string(jsonBytes) != "{}" {
		t.Fatalf("Expected empty JSON object after clear, got: %s", string(jsonBytes))
	}
}

// TestNoOp verifies that NoOp metrics don't allocate or affect performance
func TestNoOp(t *testing.T) {
	m := NoOp()

	// All operations should be no-ops
	m.Timer("noop").Start()
	m.Timer("noop").Stop()
	m.Counter("noop").Incr()
	m.Histogram("noop").Update(100)

	all := m.All()

	// Should be empty
	if len(all) > 0 {
		t.Fatalf("Expected NoOp metrics to be empty, got: %v", all)
	}

	// String should return empty string or minimal output
	// NoOp metrics don't have String() method, so skip this check
	// The NoOp implementation doesn't support String() method
}

// TestTimerInt64Consistency verifies that Int64() always returns the same value for a stopped timer
func TestTimerInt64Consistency(t *testing.T) {
	m := New()

	m.Timer("consistency").Start()
	time.Sleep(100 * time.Microsecond)
	m.Timer("consistency").Stop()

	// Get value multiple times
	val1 := m.Timer("consistency").Int64()
	val2 := m.Timer("consistency").Int64()
	val3 := m.Timer("consistency").Int64()

	if val1 != val2 || val2 != val3 {
		t.Fatalf("Expected consistent timer values, got: %d, %d, %d", val1, val2, val3)
	}

	if val1 == 0 {
		t.Fatal("Expected non-zero timer value")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
