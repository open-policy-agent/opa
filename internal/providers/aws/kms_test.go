package aws

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/logging"
)

func mockPayload(request KMSSignRequest) string {
	responseFmt := `{"KeyId": "%s", "Signature": "%s", "SigningAlgorithm": "%s"}`
	return fmt.Sprintf(responseFmt, request.KeyID, request.Message, request.SigningAlgorithm)
}

func TestKMS_SignDigest(t *testing.T) {
	type testCase struct {
		name            string
		request         KMSSignRequest
		responsePayload string
		responseStatus  int
		wantSignature   string
		wantErr         bool
	}

	run := func(t *testing.T, tc testCase) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tc.responseStatus != 200 {
				w.WriteHeader(tc.responseStatus)
			}
			if _, err := io.WriteString(w, tc.responsePayload); err != nil {
				t.Fatalf("io.WriteString(w, payload) = %v", err)
			}

		}))
		defer server.Close()

		logger := logging.New()
		logger.SetLevel(logging.Debug)

		kms := NewKMSWithURLClient(server.URL, server.Client(), logger)

		creds := Credentials{}
		signature, err := kms.SignDigest(context.Background(), []byte(tc.request.Message), tc.request.KeyID, tc.request.SigningAlgorithm, creds, "v4")
		if err != nil && tc.wantErr == false {
			t.Fatalf("expected no error, got: %s", err)
		}

		if err == nil && tc.wantErr {
			t.Fatal("expected error")
		}

		if err == nil && tc.wantSignature != signature {
			t.Fatalf("expected %s, got %s", tc.wantSignature, signature)
		}

	}
	validRequest1 := KMSSignRequest{
		KeyID:            "Keyid1",
		Message:          "sample",
		SigningAlgorithm: "ECDSA_SHA_256",
	}
	testCases := []testCase{
		{
			name:            "valid response",
			request:         validRequest1,
			responsePayload: mockPayload(validRequest1),
			responseStatus:  200,
			wantSignature:   validRequest1.Message,
			wantErr:         false,
		},
		{
			name:            "error response",
			request:         validRequest1,
			responsePayload: "Backend error",
			responseStatus:  500,
			wantErr:         true,
		},
		{
			name:            "valid error response",
			request:         validRequest1,
			responsePayload: `{ "__type" :"SerializationException" }`,
			responseStatus:  400,
			wantErr:         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
