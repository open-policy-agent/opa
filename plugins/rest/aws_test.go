// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/sdk"
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
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	// wrong path: cred service path not well formed
	cs := awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: "this is not a URL", // malformed
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	_, err := cs.credentials()
	assertErr("unsupported protocol scheme \"\"", err, t)

	// wrong path: no role set but no ECS URI in environment
	os.Unsetenv(ecsRelativePathEnvVar)
	cs = awsMetadataCredentialService{
		RegionName: "us-east-1",
		logger:     sdk.NewStandardLogger(),
	}
	_, err = cs.credentials()
	assertErr("metadata endpoint cannot be determined from settings and environment", err, t)

	// wrong path: creds not found
	cs = awsMetadataCredentialService{
		RoleName:        "not_my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	_, err = cs.credentials()
	assertErr("metadata HTTP request returned unexpected status: 404 Not Found", err, t)

	// wrong path: malformed JSON body
	cs = awsMetadataCredentialService{
		RoleName:        "my_bad_iam_role", // not good
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	_, err = cs.credentials()
	assertErr("failed to parse credential response from metadata service: invalid character 'T' looking for beginning of value", err, t)

	// wrong path: token service error
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/missing_token",
		logger:          sdk.NewStandardLogger(),
	} // will 404
	_, err = cs.credentials()
	assertErr("metadata token HTTP request returned unexpected status: 404 Not Found", err, t)

	// wrong path: token service returns bad token
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/bad_token",
		logger:          sdk.NewStandardLogger(),
	} // not good
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
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
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
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
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
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
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
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	// wrong path: handle errors from credential service
	cs := &awsMetadataCredentialService{
		RoleName:        "not_my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	req, _ := http.NewRequest("GET", "https://mybucket.s3.amazonaws.com/bundle.tar.gz", strings.NewReader(""))
	err := signV4(req, "s3", cs, time.Unix(1556129697, 0))

	assertErr("error getting AWS credentials: metadata HTTP request returned unexpected status: 404 Not Found", err, t)

	// happy path: sign correctly
	cs = &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ = http.NewRequest("GET", "https://mybucket.s3.amazonaws.com/bundle.tar.gz", strings.NewReader(""))
	err = signV4(req, "s3", cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Fatal("unexpected error during signing")
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

func TestV4SigningForApiGateway(t *testing.T) {
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	cs := &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("POST", "https://myrestapi.execute-api.us-east-1.amazonaws.com/prod/logs",
		strings.NewReader("{ \"payload\": 42 }"))
	req.Header.Set("Content-Type", "application/json")

	err := signV4(req, "execute-api", cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Fatal("unexpected error during signing")
	}

	// expect mandatory headers
	assertEq(req.Header.Get("Host"), "myrestapi.execute-api.us-east-1.amazonaws.com", t)
	assertEq(req.Header.Get("Authorization"),
		"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/execute-api/aws4_request,"+
			"SignedHeaders=content-type;host;x-amz-date;x-amz-security-token,"+
			"Signature=c8ee72cc45050b255bcbf19defc693f7cd788959b5380fa0985de6e865635339", t)
	// no content sha should be set, since this is specific to s3 and glacier
	assertEq(req.Header.Get("X-Amz-Content-Sha256"), "", t)
	assertEq(req.Header.Get("X-Amz-Date"), "20190424T181457Z", t)
	assertEq(req.Header.Get("X-Amz-Security-Token"), "MYAWSSECURITYTOKENGOESHERE", t)
}

func TestV4SigningOmitsIgnoredHeaders(t *testing.T) {
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	cs := &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("POST", "https://myrestapi.execute-api.us-east-1.amazonaws.com/prod/logs",
		strings.NewReader("{ \"payload\": 42 }"))
	req.Header.Set("Content-Type", "application/json")

	// These are headers that should never be included in the signed headers
	req.Header.Set("User-Agent", "Unit Tests!")
	req.Header.Set("Authorization", "Auth header will be overwritten, and shouldn't be signed")
	req.Header.Set("X-Amzn-Trace-Id", "Some trace id")

	err := signV4(req, "execute-api", cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Fatal("unexpected error during signing")
	}

	// Check the signed headers doesn't include user-agent, authorization or x-amz-trace-id
	assertEq(req.Header.Get("Authorization"),
		"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/execute-api/aws4_request,"+
			"SignedHeaders=content-type;host;x-amz-date;x-amz-security-token,"+
			"Signature=c8ee72cc45050b255bcbf19defc693f7cd788959b5380fa0985de6e865635339", t)
	// The headers omitted from signing should still be present in the request
	assertEq(req.Header.Get("User-Agent"), "Unit Tests!", t)
	assertEq(req.Header.Get("X-Amzn-Trace-Id"), "Some trace id", t)
}

func TestV4SigningCustomPort(t *testing.T) {
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	cs := &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("GET", "https://custom.s3.server:9000/bundle.tar.gz", strings.NewReader(""))
	err := signV4(req, "s3", cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Fatal("unexpected error during signing")
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

func TestV4SigningDoesNotMutateBody(t *testing.T) {
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	cs := &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("POST", "https://myrestapi.execute-api.us-east-1.amazonaws.com/prod/logs",
		strings.NewReader("{ \"payload\": 42 }"))

	err := signV4(req, "execute-api", cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Fatal("unexpected error during signing")
	}

	// Read the body and check that it was not mutated
	body, _ := ioutil.ReadAll(req.Body)
	assertEq(string(body), "{ \"payload\": 42 }", t)
}

func TestV4SigningWithMultiValueHeaders(t *testing.T) {
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	cs := &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          sdk.NewStandardLogger(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("POST", "https://myrestapi.execute-api.us-east-1.amazonaws.com/prod/logs",
		strings.NewReader("{ \"payload\": 42 }"))
	req.Header.Add("Accept", "text/plain")
	req.Header.Add("Accept", "text/html")

	err := signV4(req, "execute-api", cs, time.Unix(1556129697, 0))

	if err != nil {
		t.Fatal("unexpected error during signing")
	}

	// Check the signed headers includes our multi-value 'accept' header
	assertEq(req.Header.Get("Authorization"),
		"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/execute-api/aws4_request,"+
			"SignedHeaders=accept;host;x-amz-date;x-amz-security-token,"+
			"Signature=0237b0c789cad36212f0efba70c02549e1f659ab9caaca16423930cc7236c046", t)
	// The multi-value headers are preserved
	assertEq(req.Header.Values("Accept")[0], "text/plain", t)
	assertEq(req.Header.Values("Accept")[1], "text/html", t)
}

// simulate EC2 metadata service
type ec2CredTestServer struct {
	t         *testing.T
	server    *httptest.Server
	expPath   string
	expMethod string
	payload   metadataPayload // must set before use
}

func (t *ec2CredTestServer) handle(w http.ResponseWriter, r *http.Request) {
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

func (t *ec2CredTestServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *ec2CredTestServer) stop() {
	t.server.Close()
}

func TestWebIdentityCredentialService(t *testing.T) {
	testAccessKey := "ASgeIAIOSFODNN7EXAMPLE"
	ts := stsTestServer{
		t:         t,
		accessKey: testAccessKey,
	}
	ts.start()
	defer ts.stop()
	cs := awsWebIdentityCredentialService{
		stsURL: ts.server.URL,
		logger: sdk.NewStandardLogger(),
	}

	goodTokenFile, err := ioutil.TempFile(os.TempDir(), "opa-aws-test-")
	if err != nil {
		t.Errorf("Error while creating token file: %s", err)
		return
	}
	defer os.Remove(goodTokenFile.Name())
	goodTokenFile.WriteString("good-token")
	goodTokenFile.Close()

	badTokenFile, err := ioutil.TempFile(os.TempDir(), "opa-aws-test-")
	if err != nil {
		t.Errorf("Error while creating token file: %s", err)
		return
	}
	defer os.Remove(badTokenFile.Name())
	badTokenFile.WriteString("bad-token")
	badTokenFile.Close()

	// wrong path: no AWS_ROLE_ARN set
	err = cs.populateFromEnv()
	assertErr("no AWS_ROLE_ARN set in environment", err, t)
	os.Setenv("AWS_ROLE_ARN", "role:arn")

	// wrong path: no AWS_WEB_IDENTITY_TOKEN_FILE set
	err = cs.populateFromEnv()
	assertErr("no AWS_WEB_IDENTITY_TOKEN_FILE set in environment", err, t)
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/nonsense")

	// happy path: both env vars set
	err = cs.populateFromEnv()
	if err != nil {
		t.Errorf("Error while getting env vars: %s", err)
		return
	}

	// wrong path: refresh with invalid web token file
	err = cs.refreshFromService()
	assertErr("unable to read web token for sts HTTP request: open /nonsense: no such file or directory", err, t)

	// wrong path: refresh with "bad token"
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", badTokenFile.Name())
	_ = cs.populateFromEnv()
	err = cs.refreshFromService()
	assertErr("STS HTTP request returned unexpected status: 401 Unauthorized", err, t)

	// happy path: refresh with "good token"
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", goodTokenFile.Name())
	_ = cs.populateFromEnv()
	err = cs.refreshFromService()
	if err != nil {
		t.Fatalf("Unexpected err: %s", err)
	}

	// happy path: refresh and get credentials
	creds, _ := cs.credentials()
	assertEq(creds.AccessKey, testAccessKey, t)

	// happy path: refresh with session and get credentials
	cs.expiration = time.Now()
	cs.SessionName = "TEST_SESSION"
	creds, _ = cs.credentials()
	assertEq(creds.AccessKey, testAccessKey, t)

	// happy path: don't refresh, but get credentials
	ts.accessKey = "OTHERKEY"
	creds, _ = cs.credentials()
	assertEq(creds.AccessKey, testAccessKey, t)

	// happy/wrong path: refresh with "bad token" but return previous credentials
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", badTokenFile.Name())
	_ = cs.populateFromEnv()
	cs.expiration = time.Now()
	creds, err = cs.credentials()
	assertEq(creds.AccessKey, testAccessKey, t)
	assertErr("STS HTTP request returned unexpected status: 401 Unauthorized", err, t)

	// wrong path: refresh with "bad token" but return previous credentials
	os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", goodTokenFile.Name())
	os.Setenv("AWS_ROLE_ARN", "BrokenRole")
	_ = cs.populateFromEnv()
	cs.expiration = time.Now()
	creds, err = cs.credentials()
	assertErr("failed to parse credential response from STS service: EOF", err, t)
}

func TestStsPath(t *testing.T) {
	cs := awsWebIdentityCredentialService{}

	assertEq(cs.stsPath(), stsDefaultPath, t)

	cs.RegionName = "us-east-2"
	assertEq(cs.stsPath(), "https://sts.us-east-2.amazonaws.com", t)

	cs.stsURL = "http://test.com"
	assertEq(cs.stsPath(), "http://test.com", t)
}

// simulate EC2 metadata service
type stsTestServer struct {
	t         *testing.T
	server    *httptest.Server
	accessKey string
}

func (t *stsTestServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || r.URL.Query().Get("Action") != "AssumeRoleWithWebIdentity" {
		w.WriteHeader(404)
		return
	}

	if r.URL.Query().Get("RoleArn") == "BrokenRole" {
		w.WriteHeader(200)
		w.Write([]byte("{}"))
		return
	}

	token := r.URL.Query().Get("WebIdentityToken")
	if token != "good-token" {
		w.WriteHeader(401)
		return
	}
	w.WriteHeader(200)

	sessionName := r.URL.Query().Get("RoleSessionName")

	// Taken from STS docs: https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html
	xmlResponse := `<AssumeRoleWithWebIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
	<AssumeRoleWithWebIdentityResult>
	  <SubjectFromWebIdentityToken>amzn1.account.AF6RHO7KZU5XRVQJGXK6HB56KR2A</SubjectFromWebIdentityToken>
	  <Audience>client.5498841531868486423.1548@apps.example.com</Audience>
	  <AssumedRoleUser>
		<Arn>arn:aws:sts::123456789012:assumed-role/FederatedWebIdentityRole/%[1]s</Arn>
		<AssumedRoleId>AROACLKWSDQRAOEXAMPLE:%[1]s</AssumedRoleId>
	  </AssumedRoleUser>
	  <Credentials>
		<SessionToken>AQoDYXdzEE0a8ANXXXXXXXXNO1ewxE5TijQyp+IEXAMPLE</SessionToken>
		<SecretAccessKey>wJalrXUtnFEMI/K7MDENG/bPxRfiCYzEXAMPLEKEY</SecretAccessKey>
		<Expiration>%s</Expiration>
		<AccessKeyId>%s</AccessKeyId>
	  </Credentials>
	  <Provider>www.amazon.com</Provider>
	</AssumeRoleWithWebIdentityResult>
	<ResponseMetadata>
	  <RequestId>ad4156e9-bce1-11e2-82e6-6b6efEXAMPLE</RequestId>
	</ResponseMetadata>
  </AssumeRoleWithWebIdentityResponse>`

	w.Write([]byte(fmt.Sprintf(xmlResponse, sessionName, time.Now().Add(time.Hour).Format(time.RFC3339), t.accessKey)))
}

func (t *stsTestServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *stsTestServer) stop() {
	t.server.Close()
}
