// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// this is usually private; but we need it here
type metadataPayload struct {
	Code            string
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string
	Token           string
	Expiration      time.Time
}

// quicky and dirty assertions
func assertEq(expected string, actual string, t *testing.T) {
	t.Helper()
	if actual != expected {
		t.Error("expected: ", expected, " but got: ", actual)
	}
}

func assertErr(expected string, actual error, t *testing.T) {
	t.Helper()
	if !strings.Contains(actual.Error(), expected) {
		t.Errorf("Expected error to contain %s, got: %s", expected, actual.Error())
	}
}

func TestEnvironmentCredentialService(t *testing.T) {
	os.Setenv("AWS_ACCESS_KEY_ID", "")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "")
	os.Setenv("AWS_REGION", "")
	os.Setenv("AWS_SECURITY_TOKEN", "")
	os.Setenv("AWS_SESSION_TOKEN", "")

	cs := &awsEnvironmentCredentialService{}

	// wrong path: some required environment is missing
	envCreds, err := cs.credentials()
	assertErr("no AWS_ACCESS_KEY_ID set in environment", err, t)

	os.Setenv("AWS_ACCESS_KEY_ID", "MYAWSACCESSKEYGOESHERE")
	envCreds, err = cs.credentials()
	assertErr("no AWS_SECRET_ACCESS_KEY set in environment", err, t)

	os.Setenv("AWS_SECRET_ACCESS_KEY", "MYAWSSECRETACCESSKEYGOESHERE")
	envCreds, err = cs.credentials()
	assertErr("no AWS_REGION set in environment", err, t)

	os.Setenv("AWS_REGION", "us-east-1")

	expectedCreds := awsCredentials{
		AccessKey:    "MYAWSACCESSKEYGOESHERE",
		SecretKey:    "MYAWSSECRETACCESSKEYGOESHERE",
		RegionName:   "us-east-1",
		SessionToken: ""}

	testCases := []struct {
		tokenEnv   string
		tokenValue string
	}{
		// happy path: all required environment is present
		{"", ""},
		// happy path: all required environment is present including security token
		{"AWS_SECURITY_TOKEN", "MYSECURITYTOKENGOESHERE"},
		// happy path: all required environment is present including session token that is preferred over security token
		{"AWS_SESSION_TOKEN", "MYSESSIONTOKENGOESHERE"},
	}

	for _, testCase := range testCases {
		os.Setenv(testCase.tokenEnv, testCase.tokenValue)
		expectedCreds.SessionToken = testCase.tokenValue

		envCreds, err = cs.credentials()
		if err != nil {
			t.Error("unexpected error: " + err.Error())
		}

		if envCreds != expectedCreds {
			t.Error("expected: ", expectedCreds, " but got: ", envCreds)
		}
	}
}

func TestMetadataCredentialService(t *testing.T) {
	ts := credTestServer{}
	ts.start()
	defer ts.stop()

	// wrong path: cred service path not well formed
	cs := awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: "this is not a URL", // malformed
		tokenPath:       ts.server.URL + "/latest/api/token"}
	_, err := cs.credentials()
	assertErr("unsupported protocol scheme \"\"", err, t)

	// wrong path: no role set but no ECS URI in environment
	os.Unsetenv(ecsRelativePathEnvVar)
	cs = awsMetadataCredentialService{
		RegionName: "us-east-1"}
	_, err = cs.credentials()
	assertErr("metadata endpoint cannot be determined from settings and environment", err, t)

	// wrong path: creds not found
	cs = awsMetadataCredentialService{
		RoleName:        "not_my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	_, err = cs.credentials()
	assertErr("metadata HTTP request returned unexpected status: 404 Not Found", err, t)

	// wrong path: malformed JSON body
	cs = awsMetadataCredentialService{
		RoleName:        "my_bad_iam_role", // not good
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	_, err = cs.credentials()
	assertErr("failed to parse credential response from metadata service: invalid character 'T' looking for beginning of value", err, t)

	// wrong path: token service error
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/missing_token"} // will 404
	_, err = cs.credentials()
	assertErr("metadata token HTTP request returned unexpected status: 404 Not Found", err, t)

	// wrong path: token service returns bad token
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/bad_token"} // not good
	_, err = cs.credentials()
	assertErr("metadata HTTP request returned unexpected status: 401 Unauthorized", err, t)

	// wrong path: bad result code from EC2 metadata service
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Failure", // this is bad
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 30)}
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	_, err = cs.credentials()
	assertErr("metadata service query did not succeed: Failure", err, t)

	// happy path: base case
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 300)}
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	var creds awsCredentials
	creds, err = cs.credentials()

	assertEq(creds.AccessKey, ts.payload.AccessKeyID, t)
	assertEq(creds.SecretKey, ts.payload.SecretAccessKey, t)
	assertEq(creds.RegionName, cs.RegionName, t)
	assertEq(creds.SessionToken, ts.payload.Token, t)

	// happy path: verify credentials are cached based on expiry
	ts.payload.AccessKeyID = "ICHANGEDTHISBUTWEWONTSEEIT"
	creds, err = cs.credentials()

	assertEq(creds.AccessKey, "MYAWSACCESSKEYGOESHERE", t) // the original value
	assertEq(creds.SecretKey, ts.payload.SecretAccessKey, t)
	assertEq(creds.RegionName, cs.RegionName, t)
	assertEq(creds.SessionToken, ts.payload.Token, t)

	// happy path: with refresh
	// first time through
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)} // short time

	creds, err = cs.credentials()

	assertEq(creds.AccessKey, ts.payload.AccessKeyID, t)
	assertEq(creds.SecretKey, ts.payload.SecretAccessKey, t)
	assertEq(creds.RegionName, cs.RegionName, t)
	assertEq(creds.SessionToken, ts.payload.Token, t)

	// second time through, with changes
	ts.payload.AccessKeyID = "ICHANGEDTHISANDWEWILLSEEIT"
	creds, err = cs.credentials()

	assertEq(creds.AccessKey, ts.payload.AccessKeyID, t) // the new value
	assertEq(creds.SecretKey, ts.payload.SecretAccessKey, t)
	assertEq(creds.RegionName, cs.RegionName, t)
	assertEq(creds.SessionToken, ts.payload.Token, t)
}

type testCredentialService struct{}

func (cs *testCredentialService) credentials() (awsCredentials, error) {
	return awsCredentials{AccessKey: "MYAWSACCESSKEYGOESHERE",
		SecretKey:    "MYAWSSECRETACCESSKEYGOESHERE",
		RegionName:   "us-east-1",
		SessionToken: "MYAWSSECURITYTOKENGOESHERE"}, nil
}

func TestV4Signing(t *testing.T) {
	ts := credTestServer{}
	ts.start()
	defer ts.stop()

	// wrong path: handle errors from credential service
	cs := &awsMetadataCredentialService{
		RoleName:        "not_my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	req, _ := http.NewRequest("GET", "https://mybucket.s3.amazonaws.com/bundle.tar.gz", strings.NewReader(""))
	err := signV4(req, cs, time.Unix(1556129697, 0))

	assertErr("error getting AWS credentials: metadata HTTP request returned unexpected status: 404 Not Found", err, t)

	// happy path: sign correctly
	cs = &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ = http.NewRequest("GET", "https://mybucket.s3.amazonaws.com/bundle.tar.gz", strings.NewReader(""))
	err = signV4(req, cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Error("unexpected error during signing")
	}

	// expect mandatory headers
	assertEq(req.Header.Get("Host"), "mybucket.s3.amazonaws.com", t)
	assertEq(req.Header.Get("Authorization"),
		"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/s3/aws4_request,"+
			"SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token,"+
			"Signature=d3f0561abae5e35d9ee2c15e678bb7acacc4b4743707a8f7fbcbfdb519078990", t)
	assertEq(req.Header.Get("X-Amz-Content-Sha256"),
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", t)
	assertEq(req.Header.Get("X-Amz-Date"), "20190424T181457Z", t)
	assertEq(req.Header.Get("X-Amz-Security-Token"), "MYAWSSECURITYTOKENGOESHERE", t)
}

func TestV4SigningCustomPort(t *testing.T) {
	ts := credTestServer{}
	ts.start()
	defer ts.stop()

	cs := &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token"}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("GET", "https://custom.s3.server:9000/bundle.tar.gz", strings.NewReader(""))
	err := signV4(req, cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Error("unexpected error during signing")
	}

	// expect mandatory headers
	assertEq(req.Header.Get("Host"), "custom.s3.server:9000", t)
	assertEq(req.Header.Get("Authorization"),
		"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/s3/aws4_request,"+
			"SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token,"+
			"Signature=765b67c6b136f99d9b769171c9939fc444021f7d17e4fbe6e1ab8b1926713c2b", t)
	assertEq(req.Header.Get("X-Amz-Content-Sha256"),
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", t)
	assertEq(req.Header.Get("X-Amz-Date"), "20190424T181457Z", t)
	assertEq(req.Header.Get("X-Amz-Security-Token"), "MYAWSSECURITYTOKENGOESHERE", t)
}

// simulate EC2 metadata service
type credTestServer struct {
	t         *testing.T
	server    *httptest.Server
	expPath   string
	expMethod string
	payload   metadataPayload // must set before use
}

func (t *credTestServer) handle(w http.ResponseWriter, r *http.Request) {
	goodPath := "/latest/meta-data/iam/security-credentials/my_iam_role"
	badPath := "/latest/meta-data/iam/security-credentials/my_bad_iam_role"

	goodTokenPath := "/latest/api/token"
	badTokenPath := "/latest/api/bad_token"

	tokenValue := "THIS_IS_A_GOOD_TOKEN"
	jsonBytes, _ := json.Marshal(t.payload)

	switch r.URL.Path {
	case goodTokenPath:
		// a valid token
		w.WriteHeader(200)
		w.Write([]byte(tokenValue))
	case badTokenPath:
		// an invalid token
		w.WriteHeader(200)
		w.Write([]byte("THIS_IS_A_BAD_TOKEN"))
	case goodPath:
		// validate token...
		if r.Header.Get("X-aws-ec2-metadata-token") == tokenValue {
			// a metadata response that's well-formed
			w.WriteHeader(200)
			w.Write(jsonBytes)
		} else {
			// an unauthorized response
			w.WriteHeader(401)
		}
	case badPath:
		// a metadata response that's not well-formed
		w.WriteHeader(200)
		w.Write([]byte("This isn't a JSON payload"))
	default:
		// something else that we won't be able to find
		w.WriteHeader(404)
	}
}

func (t *credTestServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *credTestServer) stop() {
	t.server.Close()
}
