package aws

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/logging"
)

func TestECR(t *testing.T) {
	payload := `{
		"authorizationData": [
			{
				"authorizationToken": "secret",
				"expiresAt": 1.676258918209E9
			}
		]
	}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, payload); err != nil {
			t.Fatalf("io.WriteString(w, payload) = %v", err)
		}
	}))
	defer server.Close()

	logger := logging.New()
	logger.SetLevel(logging.Debug)

	ecr := ECR{
		endpoint: func(string) string { return server.URL },
		client:   server.Client(),
		logger:   logger,
	}

	creds := Credentials{}
	token, err := ecr.GetAuthorizationToken(context.Background(), creds, "v4")
	if err != nil {
		t.Errorf("ecrServer.getAuthorizationToken = %v", err)
	}

	if token.AuthorizationToken != "secret" {
		t.Errorf("token.AuthorizationToken = %q, want = %q", token.AuthorizationToken, "secret")
	}

	got := token.ExpiresAt
	want := time.Date(2023, 02, 13, 03, 28, 38, 209*1000*1000, time.UTC)
	if !got.Equal(want) {
		t.Errorf("token.ExpiresAt = %v, want = %v", got, want)
	}

}

func TestParseAWSTimestamp(t *testing.T) {
	type testCase struct {
		name       string
		raw        json.Number
		wantParsed time.Time
		wantErr    bool
	}

	run := func(t *testing.T, tc testCase) {
		got, err := parseTimestamp(tc.raw)
		if err != nil && tc.wantErr == false {
			t.Fatalf("expected no error, got: %s", err)
		}

		if err == nil && tc.wantErr {
			t.Fatal("expected error")
		}

		if !tc.wantParsed.Equal(got) {
			t.Fatalf("expected %s, got %s", tc.wantParsed, got)
		}
	}

	testCases := []testCase{
		{
			name:    "empty raw value",
			wantErr: true,
		},
		{
			name:    "no number",
			raw:     "no-number",
			wantErr: true,
		},
		{
			name:       "float in e notation",
			raw:        "1.676258918209E9", // actual timestamp returned from the API
			wantParsed: time.Date(2023, 02, 13, 03, 28, 38, 209*1000*1000, time.UTC),
		},
		{
			name:       "raw float",
			raw:        "1676258918.209",
			wantParsed: time.Date(2023, 02, 13, 03, 28, 38, 209*1000*1000, time.UTC),
		},
		{
			name:       "cuts down to ms resolution",
			raw:        "1676258918.209123",
			wantParsed: time.Date(2023, 02, 13, 03, 28, 38, 209*1000*1000, time.UTC),
		},
		{
			name:       "integer without ms",
			raw:        "1676258918",
			wantParsed: time.Date(2023, 02, 13, 03, 28, 38, 0, time.UTC),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
