package ozap

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestWrapWithFields(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]interface{}
		expect []string
	}{
		{
			name: "Ex1",
			fields: map[string]interface{}{
				"bad_error":   errors.New("something went wrong"),
				"context":     "everywhere",
				"panic":       true,
				"problems":    99,
				"luftballons": int64(99),
			},
			expect: []string{
				`"bad_error":"something went wrong"`,
				`"context":"everywhere"`,
				`"panic":true`,
				`"problems":99`,
				`"luftballons":99`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			level := zap.NewAtomicLevelAt(zap.InfoLevel)
			zaplogger := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()), zapcore.AddSync(&buf), zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
				return lvl >= level.Level()
			})))
			Wrap(zaplogger, &level).WithFields(tt.fields).Info("test")
			out := buf.String()
			for _, e := range tt.expect {
				if !strings.Contains(out, e) {
					t.Fail()
					log.Printf("Missing %s in %s", e, out)
				}
			}
		})
	}
}
