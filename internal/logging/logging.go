// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/logging"
)

func GetLevel(level string) (logging.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return logging.Debug, nil
	case "", "info":
		return logging.Info, nil
	case "warn":
		return logging.Warn, nil
	case "error":
		return logging.Error, nil
	default:
		return logging.Debug, fmt.Errorf("invalid log level: %v", level)
	}
}

func GetFormatter(format, timestampFormat string) logrus.Formatter {
	switch format {
	case "text":
		return &prettyFormatter{}
	case "json-pretty":
		return &logrus.JSONFormatter{PrettyPrint: true, TimestampFormat: timestampFormat}
	default:
		return &logrus.JSONFormatter{TimestampFormat: timestampFormat}
	}
}

// prettyFormatter implements the Logrus Formatter interface
// and provides a more simple, but easier to read, text formatter
// option than the default logrus.TextFormatter.
type prettyFormatter struct {
}

func isJSON(buf []byte) bool {
	var tmp interface{}
	err := json.Unmarshal(buf, &tmp)
	return err == nil
}

func spaces(num int) string {
	sb := strings.Builder{}
	for i := 0; i < num; i++ {
		sb.WriteByte(' ')
	}
	return sb.String()
}

func (p *prettyFormatter) Format(e *logrus.Entry) ([]byte, error) {
	b := new(bytes.Buffer)

	level := strings.ToUpper(e.Level.String())
	b.WriteString(fmt.Sprintf("[%s] %s\n", level, e.Message))

	// Format each key for optimal ease of human reading
	fieldIndent := 2
	multiLineIndent := 6
	for k, v := range e.Data {
		// Special case for multi-line strings, keep them as-is
		// but indent them. Everything else gets json'd
		stringVal, ok := v.(string)
		if ok && strings.Contains(stringVal, "\n") {
			sb := strings.Builder{}
			for i, line := range strings.Split(stringVal, "\n") {
				// match the json indent helper by not indenting the first value
				if i != 0 {
					sb.WriteString(spaces(multiLineIndent))
				}
				sb.WriteString(line)
				sb.WriteByte('\n')
				stringVal = sb.String()
			}
		} else if ok && isJSON([]byte(stringVal)) {
			var tmp bytes.Buffer
			err := json.Indent(&tmp, []byte(stringVal), spaces(multiLineIndent), spaces(2))
			if err != nil {
				return nil, err
			}
			stringVal = tmp.String()
		} else {
			jsonVal, err := json.MarshalIndent(v, spaces(multiLineIndent), spaces(2))
			if err != nil {
				return nil, err
			}
			stringVal = string(jsonVal)
		}

		b.WriteString(spaces(fieldIndent))
		b.WriteString(k)
		if strings.Contains(stringVal, "\n") {
			b.WriteString(" = |\n")
			b.WriteString(spaces(multiLineIndent))
		} else {
			b.WriteString(" = ")
		}
		b.WriteString(stringVal)
		b.WriteString("\n")
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
