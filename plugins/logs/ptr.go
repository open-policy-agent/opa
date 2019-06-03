package logs

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/util"
)

type ptr []string

func parsePtr(str string) (ptr, error) {

	if len(str) == 0 {
		return nil, fmt.Errorf("mask must be non-empty")
	} else if str[0] != '/' {
		return nil, fmt.Errorf("mask must be slash-prefixed")
	}

	parts := strings.Split(str[1:], "/")

	if parts[0] != "input" && parts[0] != "result" {
		return nil, fmt.Errorf("mask prefix not allowed")
	}

	for i := range parts {
		var err error
		parts[i], err = url.QueryUnescape(parts[i])
		if err != nil {
			return nil, err
		}
	}

	return ptr(parts), nil
}

func (p ptr) String() string {

	escaped := make([]string, len(p))

	for i := range escaped {
		escaped[i] = url.QueryEscape(p[i])
	}

	return "/" + strings.Join(escaped, "/")
}

func (p ptr) Erase(event *EventV1) {

	if len(p) == 1 {
		switch p[0] {
		case "input":
			event.Input = nil
		case "result":
			event.Result = nil
		default:
			panic("illegal value")
		}
	} else {
		var node interface{}

		switch p[0] {
		case "input":
			if event.Input != nil {
				node = *event.Input
			}
		case "result":
			if event.Result != nil {
				node = *event.Result
			}
		}

		parent := p[1 : len(p)-1].lookup(node)
		parentObj, ok := parent.(map[string]interface{})
		if !ok {
			return
		}

		fld := p[len(p)-1]
		if _, ok := parentObj[fld]; !ok {
			return
		}

		delete(parentObj, fld)
	}

	event.Erased = append(event.Erased, p.String())
}

func (p ptr) lookup(node interface{}) interface{} {
	for i := 0; i < len(p); i++ {
		switch v := node.(type) {
		case map[string]interface{}:
			var ok bool
			if node, ok = v[p[i]]; !ok {
				return nil
			}
		case []interface{}:
			idx, err := strconv.Atoi(p[i])
			if err != nil {
				return nil
			} else if idx < 0 || idx >= len(v) {
				return nil
			}
			node = v[idx]
		default:
			return nil
		}
	}

	return node
}

func resultValueToPtrs(rv interface{}) ([]ptr, error) {

	bs, err := json.Marshal(rv)
	if err != nil {
		return nil, err
	}

	var value []string

	if err := util.Unmarshal(bs, &value); err != nil {
		return nil, err
	}

	result := make([]ptr, len(value))

	for i := range result {
		if result[i], err = parsePtr(value[i]); err != nil {
			return nil, err
		}
	}

	return result, nil
}
