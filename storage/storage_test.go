package storage_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func TestNonEmpty(t *testing.T) {

	cases := []struct {
		content string
		path    string
		exp     bool
	}{
		{
			content: `{}`,
			path:    "a/b/c",
			exp:     false,
		},
		{
			content: `{"a": {}}`,
			path:    "a/b/c",
			exp:     false,
		},
		{
			content: `{"a": {"b": {}}}`,
			path:    "a/b/c",
			exp:     false,
		},
		{
			content: `{"a": {"b": {"c": {}}}}`,
			path:    "a/b/c",
			exp:     true,
		},
		{
			content: `{"a": {"b": "x"}}`,
			path:    "a/b/c",
			exp:     true,
		},
		{
			content: `{"a": "x"}`,
			path:    "a/b/c",
			exp:     true,
		},
	}

	ctx := context.Background()

	for _, tc := range cases {
		t.Run(tc.content, func(t *testing.T) {
			store := inmem.NewFromReader(bytes.NewBufferString(tc.content))
			err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
				nonEmpty, err := storage.NonEmpty(ctx, store, txn)(strings.Split(tc.path, "/"))
				if err != nil {
					t.Fatal(err)
				}
				if nonEmpty != tc.exp {
					t.Errorf("Expected %v for %v on %v but got", tc.exp, tc.path, tc.content)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}

}
