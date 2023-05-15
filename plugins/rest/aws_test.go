// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/providers/aws"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/util/test"
)

// this is usually private; but we need it here
type metadataPayload struct {
	Code            string
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string
	Token           string
	Expiration      time.Time
}

// quick and dirty assertions
func assertEq(expected string, actual string, t *testing.T) {
	t.Helper()
	if actual != expected {
		t.Error("expected: ", expected, " but got: ", actual)
	}
}
func assertIn(candidates []string, actual string, t *testing.T) {
	t.Helper()
	for _, expected := range candidates {
		if actual == expected {
			return
		}
	}
	t.Error("value: '", actual, "' not found in: ", candidates)
}

func assertErr(expected string, actual error, t *testing.T) {
	t.Helper()
	if !strings.Contains(actual.Error(), expected) {
		t.Errorf("Expected error to contain %s, got: %s", expected, actual.Error())
	}
}

func TestEnvironmentCredentialService(t *testing.T) {
	cs := &awsEnvironmentCredentialService{}

	// wrong path: some required environment is missing
	_, err := cs.credentials(context.Background())
	assertErr("no AWS_ACCESS_KEY_ID set in environment", err, t)

	t.Setenv("AWS_ACCESS_KEY_ID", "MYAWSACCESSKEYGOESHERE")
	_, err = cs.credentials(context.Background())
	assertErr("no AWS_SECRET_ACCESS_KEY set in environment", err, t)

	t.Setenv("AWS_SECRET_ACCESS_KEY", "MYAWSSECRETACCESSKEYGOESHERE")
	_, err = cs.credentials(context.Background())
	assertErr("no AWS_REGION set in environment", err, t)

	t.Setenv("AWS_REGION", "us-east-1")

	expectedCreds := aws.Credentials{
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
		if testCase.tokenEnv != "" {
			t.Setenv(testCase.tokenEnv, testCase.tokenValue)
		}
		expectedCreds.SessionToken = testCase.tokenValue

		envCreds, err := cs.credentials(context.Background())
		if err != nil {
			t.Error("unexpected error: " + err.Error())
		}

		if envCreds != expectedCreds {
			t.Error("expected: ", expectedCreds, " but got: ", envCreds)
		}
	}
}

func TestProfileCredentialService(t *testing.T) {

	defaultKey := "AKIAIOSFODNN7EXAMPLE"
	defaultSecret := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	defaultSessionToken := "AQoEXAMPLEH4aoAH0gNCAPy"
	defaultRegion := "us-west-2"

	fooKey := "AKIAI44QH8DHBEXAMPLE"
	fooSecret := "je7MtGbClwBF/2Zp9Utk/h3yCo8nvbEXAMPLEKEY"
	fooRegion := "us-east-1"

	config := fmt.Sprintf(`
[default]
aws_access_key_id=%v
aws_secret_access_key=%v
aws_session_token=%v

[foo]
aws_access_key_id=%v
aws_secret_access_key=%v
`, defaultKey, defaultSecret, defaultSessionToken, fooKey, fooSecret)

	files := map[string]string{
		"example.ini": config,
	}

	test.WithTempFS(files, func(path string) {
		cfgPath := filepath.Join(path, "example.ini")
		cs := &awsProfileCredentialService{
			Path:       cfgPath,
			Profile:    "foo",
			RegionName: fooRegion,
		}
		creds, err := cs.credentials(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		expected := aws.Credentials{
			AccessKey:    fooKey,
			SecretKey:    fooSecret,
			RegionName:   fooRegion,
			SessionToken: "",
		}

		if expected != creds {
			t.Fatalf("Expected credentials %v but got %v", expected, creds)
		}

		// "default" profile
		cs = &awsProfileCredentialService{
			Path:       cfgPath,
			Profile:    "",
			RegionName: defaultRegion,
		}

		creds, err = cs.credentials(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		expected = aws.Credentials{
			AccessKey:    defaultKey,
			SecretKey:    defaultSecret,
			RegionName:   defaultRegion,
			SessionToken: defaultSessionToken,
		}

		if expected != creds {
			t.Fatalf("Expected credentials %v but got %v", expected, creds)
		}
	})
}

func TestProfileCredentialServiceWithEnvVars(t *testing.T) {
	defaultKey := "AKIAIOSFODNN7EXAMPLE"
	defaultSecret := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	defaultSessionToken := "AQoEXAMPLEH4aoAH0gNCAPy"
	defaultRegion := "us-east-1"
	profile := "profileName"
	config := fmt.Sprintf(`
[%s]
aws_access_key_id=%s
aws_secret_access_key=%s
aws_session_token=%s
`, profile, defaultKey, defaultSecret, defaultSessionToken)

	files := map[string]string{
		"example.ini": config,
	}

	test.WithTempFS(files, func(path string) {
		cfgPath := filepath.Join(path, "example.ini")

		t.Setenv(awsCredentialsFileEnvVar, cfgPath)
		t.Setenv(awsProfileEnvVar, profile)
		t.Setenv(awsRegionEnvVar, defaultRegion)

		cs := &awsProfileCredentialService{}
		creds, err := cs.credentials(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		expected := aws.Credentials{
			AccessKey:    defaultKey,
			SecretKey:    defaultSecret,
			RegionName:   defaultRegion,
			SessionToken: defaultSessionToken,
		}

		if expected != creds {
			t.Fatalf("Expected credentials %v but got %v", expected, creds)
		}
	})
}

func TestProfileCredentialServiceWithDefaultPath(t *testing.T) {
	defaultKey := "AKIAIOSFODNN7EXAMPLE"
	defaultSecret := "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	defaultSessionToken := "AQoEXAMPLEH4aoAH0gNCAPy"
	defaultRegion := "us-west-22"

	config := fmt.Sprintf(`
[default]
aws_access_key_id=%s
aws_secret_access_key=%s
aws_session_token=%s
`, defaultKey, defaultSecret, defaultSessionToken)

	files := map[string]string{}

	test.WithTempFS(files, func(path string) {

		t.Setenv("USERPROFILE", path)
		t.Setenv("HOME", path)

		cfgDir := filepath.Join(path, ".aws")
		err := os.MkdirAll(cfgDir, os.ModePerm)
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(cfgDir, "credentials"), []byte(config), 0600); err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		cs := &awsProfileCredentialService{RegionName: defaultRegion}
		creds, err := cs.credentials(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		expected := aws.Credentials{
			AccessKey:    defaultKey,
			SecretKey:    defaultSecret,
			RegionName:   defaultRegion,
			SessionToken: defaultSessionToken,
		}

		if expected != creds {
			t.Fatalf("Expected credentials %v but got %v", expected, creds)
		}
	})
}

func TestProfileCredentialServiceWithError(t *testing.T) {
	configNoAccessKeyID := `
[default]
aws_secret_access_key = secret
`

	configNoSecret := `
[default]
aws_access_key_id=accessKey
`
	tests := []struct {
		note   string
		config string
		err    string
	}{
		{
			note:   "no aws_access_key_id",
			config: configNoAccessKeyID,
			err:    "does not contain \"aws_access_key_id\"",
		},
		{
			note:   "no aws_secret_access_key",
			config: configNoSecret,
			err:    "does not contain \"aws_secret_access_key\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			files := map[string]string{
				"example.ini": tc.config,
			}

			test.WithTempFS(files, func(path string) {
				cfgPath := filepath.Join(path, "example.ini")
				cs := &awsProfileCredentialService{
					Path: cfgPath,
				}
				_, err := cs.credentials(context.Background())
				if err == nil {
					t.Fatal("Expected error but got nil")
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Errorf("expected error to contain %v, got %v", tc.err, err.Error())
				}
			})
		})
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
		logger:          logging.Get(),
	}
	_, err := cs.credentials(context.Background())
	assertErr("unsupported protocol scheme \"\"", err, t)

	// wrong path: no role set but no ECS URI in environment
	os.Unsetenv(ecsRelativePathEnvVar)
	cs = awsMetadataCredentialService{
		RegionName: "us-east-1",
		logger:     logging.Get(),
	}
	_, err = cs.credentials(context.Background())
	assertErr("metadata endpoint cannot be determined from settings and environment", err, t)

	// wrong path: creds not found
	cs = awsMetadataCredentialService{
		RoleName:        "not_my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          logging.Get(),
	}
	_, err = cs.credentials(context.Background())
	assertErr("metadata HTTP request returned unexpected status: 404 Not Found", err, t)

	// wrong path: malformed JSON body
	cs = awsMetadataCredentialService{
		RoleName:        "my_bad_iam_role", // not good
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          logging.Get(),
	}
	_, err = cs.credentials(context.Background())
	assertErr("failed to parse credential response from metadata service: invalid character 'T' looking for beginning of value", err, t)

	// wrong path: token service error
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/missing_token",
		logger:          logging.Get(),
	} // will 404
	_, err = cs.credentials(context.Background())
	assertErr("metadata token HTTP request returned unexpected status: 404 Not Found", err, t)

	// wrong path: token service returns bad token
	cs = awsMetadataCredentialService{
		RoleName:        "my_iam_role",
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/bad_token",
		logger:          logging.Get(),
	} // not good
	_, err = cs.credentials(context.Background())
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
		logger:          logging.Get(),
	}
	_, err = cs.credentials(context.Background())
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
		logger:          logging.Get(),
	}
	var creds aws.Credentials
	creds, err = cs.credentials(context.Background())
	if err != nil {
		// Cannot proceed with test if unable to fetch credentials.
		t.Fatal(err)
	}

	assertEq(creds.AccessKey, ts.payload.AccessKeyID, t)
	assertEq(creds.SecretKey, ts.payload.SecretAccessKey, t)
	assertEq(creds.RegionName, cs.RegionName, t)
	assertEq(creds.SessionToken, ts.payload.Token, t)

	// happy path: verify credentials are cached based on expiry
	ts.payload.AccessKeyID = "ICHANGEDTHISBUTWEWONTSEEIT"
	creds, err = cs.credentials(context.Background())
	if err != nil {
		// Cannot proceed with test if unable to fetch credentials.
		t.Fatal(err)
	}

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
		logger:          logging.Get(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)} // short time

	creds, err = cs.credentials(context.Background())
	if err != nil {
		// Cannot proceed with test if unable to fetch credentials.
		t.Fatal(err)
	}

	assertEq(creds.AccessKey, ts.payload.AccessKeyID, t)
	assertEq(creds.SecretKey, ts.payload.SecretAccessKey, t)
	assertEq(creds.RegionName, cs.RegionName, t)
	assertEq(creds.SessionToken, ts.payload.Token, t)

	// second time through, with changes
	ts.payload.AccessKeyID = "ICHANGEDTHISANDWEWILLSEEIT"
	creds, err = cs.credentials(context.Background())
	if err != nil {
		// Cannot proceed with test if unable to fetch credentials.
		t.Fatal(err)
	}

	assertEq(creds.AccessKey, ts.payload.AccessKeyID, t) // the new value
	assertEq(creds.SecretKey, ts.payload.SecretAccessKey, t)
	assertEq(creds.RegionName, cs.RegionName, t)
	assertEq(creds.SessionToken, ts.payload.Token, t)
}

func TestMetadataServiceErrorHandled(t *testing.T) {
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	// wrong path: handle errors from credential service
	cs := &awsMetadataCredentialService{
		RoleName:        "not_my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          logging.Get(),
	}

	_, err := cs.credentials(context.Background())
	assertErr("metadata HTTP request returned unexpected status: 404 Not Found", err, t)
}

func TestV4Signing(t *testing.T) {
	ts := ec2CredTestServer{}
	ts.start()
	defer ts.stop()

	// happy path: sign correctly
	cs := &awsMetadataCredentialService{
		RoleName:        "my_iam_role", // not present
		RegionName:      "us-east-1",
		credServicePath: ts.server.URL + "/latest/meta-data/iam/security-credentials/",
		tokenPath:       ts.server.URL + "/latest/api/token",
		logger:          logging.Get(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("GET", "https://mybucket.s3.amazonaws.com/bundle.tar.gz", strings.NewReader(""))

	// force a non-random source so that we can predict the v4a signing key and, thus, signature
	myReader := strings.NewReader("000000000000000000000000000000000")
	aws.SetRandomSource(myReader)
	defer func() { aws.SetRandomSource(rand.Reader) }()

	tests := []struct {
		sigVersion            string
		expectedAuthorization []string
	}{
		{
			sigVersion: "4",
			expectedAuthorization: []string{
				"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/s3/aws4_request," +
					"SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-security-token," +
					"Signature=d3f0561abae5e35d9ee2c15e678bb7acacc4b4743707a8f7fbcbfdb519078990",
			},
		},
		{
			sigVersion: "4a",
			expectedAuthorization: []string{
				// this signature is for go 1.20+, which changed crypto/ecdsa so signatures differ from go 1.18
				"AWS4-ECDSA-P256-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/s3/aws4_request, " +
					"SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-region-set;x-amz-security-token, " +
					"Signature=3045022031b9dd601cd02650193586a32721d0614bf2e34bbc76cff0d9812366d1dc8878022100d0cfbd91bd2dd98f1e2d7feb9091c48f8b66a20174922770ec9e3b74db8e1826",
				// this signature is for go 1.18+. Remove this and only test for a single value when OPA drops go 1.19
				"AWS4-ECDSA-P256-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/s3/aws4_request, " +
					"SignedHeaders=host;x-amz-content-sha256;x-amz-date;x-amz-region-set;x-amz-security-token, " +
					"Signature=304402207d1bcb6fb68d85be3e9f6948a8dc8596a531b3f5a82ca2350acabe98941312bc02207d81ed07c7356226d93611820548a806c8e1f0cc72ff41ba672d23901e5a06bf",
			},
		},
	}

	for _, test := range tests {
		creds, err := cs.credentials(context.Background())
		if err != nil {
			t.Fatal("unexpected error getting credentials")
		}

		if err := aws.SignRequest(req, "s3", creds, time.Unix(1556129697, 0), test.sigVersion); err != nil {
			t.Fatal("unexpected error during signing", err)
		}

		// expect mandatory headers
		assertEq("mybucket.s3.amazonaws.com", req.Header.Get("Host"), t)
		assertIn(test.expectedAuthorization, req.Header.Get("Authorization"), t)
		assertEq("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			req.Header.Get("X-Amz-Content-Sha256"), t)
		assertEq("20190424T181457Z", req.Header.Get("X-Amz-Date"), t)
		assertEq("MYAWSSECURITYTOKENGOESHERE", req.Header.Get("X-Amz-Security-Token"), t)
	}
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
		logger:          logging.Get(),
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

	creds, err := cs.credentials(context.Background())
	if err != nil {
		t.Fatal("unexpected error getting credentials")
	}

	if err := aws.SignRequest(req, "execute-api", creds, time.Unix(1556129697, 0), "4"); err != nil {
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
		logger:          logging.Get(),
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

	// force a non-random source so that we can predict the v4a signing key and, thus, signature
	myReader := strings.NewReader("000000000000000000000000000000000")
	aws.SetRandomSource(myReader)
	defer func() { aws.SetRandomSource(rand.Reader) }()

	tests := []struct {
		sigVersion            string
		expectedAuthorization []string
	}{
		{
			sigVersion: "4",
			expectedAuthorization: []string{"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/execute-api/aws4_request," +
				"SignedHeaders=content-type;host;x-amz-date;x-amz-security-token," +
				"Signature=c8ee72cc45050b255bcbf19defc693f7cd788959b5380fa0985de6e865635339",
			},
		},
		{
			sigVersion: "4a",
			expectedAuthorization: []string{
				// this signature is for go 1.20+, which changed crypto/ecdsa so signatures differ from go 1.18
				"AWS4-ECDSA-P256-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/execute-api/aws4_request, " +
					"SignedHeaders=content-length;content-type;host;x-amz-content-sha256;x-amz-date;x-amz-region-set;x-amz-security-token, " +
					"Signature=3045022100e62b33949d5d5666c1cc737db6673600d7893b977df48e4eb64a6e8747582a2f022011f56ad285472956a3e00c6971d03ebd8ecb579804d8fd91a6fb483a1f502118",
				// this signature is for go 1.18+. Remove this and only test for a single value when OPA drops go 1.19
				"AWS4-ECDSA-P256-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/execute-api/aws4_request, " +
					"SignedHeaders=content-length;content-type;host;x-amz-content-sha256;x-amz-date;x-amz-region-set;x-amz-security-token, " +
					"Signature=30450221009f3b0cda178456dfd1bec61b78bdbd115c0cf497eaa52c58bbb2850ad9c49c3002207009cb88a1219a4a6626056c31823a6b5bc2728bc88bc98a06e12e1148482c94",
			},
		},
	}

	for _, test := range tests {
		creds, err := cs.credentials(context.Background())
		if err != nil {
			t.Fatal("unexpected error getting credentials")
		}

		if err := aws.SignRequest(req, "execute-api", creds, time.Unix(1556129697, 0), test.sigVersion); err != nil {
			t.Fatal("unexpected error during signing")
		}

		// Check the signed headers doesn't include user-agent, authorization or x-amz-trace-id
		assertIn(test.expectedAuthorization, req.Header.Get("Authorization"), t)
		// The headers omitted from signing should still be present in the request
		assertEq(req.Header.Get("User-Agent"), "Unit Tests!", t)
		assertEq(req.Header.Get("X-Amzn-Trace-Id"), "Some trace id", t)
	}

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
		logger:          logging.Get(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}
	req, _ := http.NewRequest("GET", "https://custom.s3.server:9000/bundle.tar.gz", strings.NewReader(""))

	creds, err := cs.credentials(context.Background())
	if err != nil {
		t.Fatal("unexpected error getting credentials")
	}

	if err := aws.SignRequest(req, "s3", creds, time.Unix(1556129697, 0), "4"); err != nil {
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
		logger:          logging.Get(),
	}
	ts.payload = metadataPayload{
		AccessKeyID:     "MYAWSACCESSKEYGOESHERE",
		SecretAccessKey: "MYAWSSECRETACCESSKEYGOESHERE",
		Code:            "Success",
		Token:           "MYAWSSECURITYTOKENGOESHERE",
		Expiration:      time.Now().UTC().Add(time.Minute * 2)}

	// force a non-random source so that we can predict the v4a signing key and, thus, signature
	myReader := strings.NewReader("000000000000000000000000000000000")
	aws.SetRandomSource(myReader)
	defer func() { aws.SetRandomSource(rand.Reader) }()

	tests := []struct {
		sigVersion string
	}{
		{sigVersion: "4"},
		{sigVersion: "4a"},
	}

	for _, test := range tests {
		req, _ := http.NewRequest("POST", "https://myrestapi.execute-api.us-east-1.amazonaws.com/prod/logs",
			strings.NewReader("{ \"payload\": 42 }"))

		creds, err := cs.credentials(context.Background())
		if err != nil {
			t.Fatal("unexpected error getting credentials")
		}

		if err := aws.SignRequest(req, "execute-api", creds, time.Unix(1556129697, 0), test.sigVersion); err != nil {
			t.Fatal("unexpected error during signing")
		}

		// Read the body and check that it was not mutated
		body, _ := io.ReadAll(req.Body)
		assertEq(string(body), "{ \"payload\": 42 }", t)
	}
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
		logger:          logging.Get(),
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

	// force a non-random source so that we can predict the v4a signing key and, thus, signature
	myReader := strings.NewReader("000000000000000000000000000000000")
	aws.SetRandomSource(myReader)
	defer func() { aws.SetRandomSource(rand.Reader) }()

	tests := []struct {
		sigVersion            string
		expectedAuthorization []string
	}{
		{
			sigVersion: "4",
			expectedAuthorization: []string{
				"AWS4-HMAC-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/us-east-1/execute-api/aws4_request," +
					"SignedHeaders=accept;host;x-amz-date;x-amz-security-token," +
					"Signature=0237b0c789cad36212f0efba70c02549e1f659ab9caaca16423930cc7236c046",
			},
		},
		{
			sigVersion: "4a",
			expectedAuthorization: []string{
				// this signature is for go 1.20+, which changed crypto/ecdsa so signatures differ from go 1.18
				"AWS4-ECDSA-P256-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/execute-api/aws4_request, " +
					"SignedHeaders=accept;content-length;host;x-amz-content-sha256;x-amz-date;x-amz-region-set;x-amz-security-token, " +
					"Signature=3046022100f7fd07e2a00b1be3074be0c2e3871bd42ddc4c01549b1ffc4809ef3fafde80780221008c6bf906cdb9040ebeb94d1134598e7920fa8cb7bda91b00ce0ab9838b79631b",
				// this signature is for go 1.18+. Remove this and only test for a single value when OPA drops go 1.19
				"AWS4-ECDSA-P256-SHA256 Credential=MYAWSACCESSKEYGOESHERE/20190424/execute-api/aws4_request, " +
					"SignedHeaders=accept;content-length;host;x-amz-content-sha256;x-amz-date;x-amz-region-set;x-amz-security-token, " +
					"Signature=304402202d5f2d4d42fe59b2e61fa455cb35a335139d109c2d37aaa8946d45fd0fb4989c022068238cbfbc80326f5cc391f2b6837910191ceabb58ec0bf986c0141f76046594",
			},
		},
	}

	for _, test := range tests {
		creds, err := cs.credentials(context.Background())
		if err != nil {
			t.Fatal("unexpected error getting credentials")
		}

		if err := aws.SignRequest(req, "execute-api", creds, time.Unix(1556129697, 0), test.sigVersion); err != nil {
			t.Fatal("unexpected error during signing")
		}

		if len(req.Header.Values("Authorization")) != 1 {
			t.Fatal("Authorization header is multi-valued. This will break AWS v4 signing.")
		}
		// Check the signed headers includes our multi-value 'accept' header
		assertIn(test.expectedAuthorization, req.Header.Get("Authorization"), t)
		// The multi-value headers are preserved
		assertEq("text/plain", req.Header.Values("Accept")[0], t)
		assertEq("text/html", req.Header.Values("Accept")[1], t)
	}
}

// simulate EC2 metadata service
type ec2CredTestServer struct {
	server  *httptest.Server
	payload metadataPayload // must set before use
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
		_, _ = w.Write([]byte(tokenValue))
	case badTokenPath:
		// an invalid token
		w.WriteHeader(200)
		_, _ = w.Write([]byte("THIS_IS_A_BAD_TOKEN"))
	case goodPath:
		// validate token...
		if r.Header.Get("X-aws-ec2-metadata-token") == tokenValue {
			// a metadata response that's well-formed
			w.WriteHeader(200)
			_, _ = w.Write(jsonBytes)
		} else {
			// an unauthorized response
			w.WriteHeader(401)
		}
	case badPath:
		// a metadata response that's not well-formed
		w.WriteHeader(200)
		_, _ = w.Write([]byte("This isn't a JSON payload"))
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
	t.Setenv("AWS_REGION", "us-west-1")

	testAccessKey := "ASgeIAIOSFODNN7EXAMPLE"
	ts := stsTestServer{
		t:         t,
		accessKey: testAccessKey,
	}
	ts.start()
	defer ts.stop()
	cs := awsWebIdentityCredentialService{
		stsURL: ts.server.URL,
		logger: logging.Get(),
	}

	files := map[string]string{
		"good_token_file": "good-token",
		"bad_token_file":  "bad-token",
	}

	test.WithTempFS(files, func(path string) {
		goodTokenFile := filepath.Join(path, "good_token_file")
		badTokenFile := filepath.Join(path, "bad_token_file")

		// wrong path: no AWS_ROLE_ARN set
		err := cs.populateFromEnv()
		assertErr("no AWS_ROLE_ARN set in environment", err, t)
		t.Setenv("AWS_ROLE_ARN", "role:arn")

		// wrong path: no AWS_WEB_IDENTITY_TOKEN_FILE set
		err = cs.populateFromEnv()
		assertErr("no AWS_WEB_IDENTITY_TOKEN_FILE set in environment", err, t)
		t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/nonsense")

		// happy path: both env vars set
		err = cs.populateFromEnv()
		if err != nil {
			t.Fatalf("Error while getting env vars: %s", err)
		}

		// wrong path: refresh with invalid web token file
		err = cs.refreshFromService(context.Background())
		assertErr("unable to read web token for sts HTTP request: open /nonsense: no such file or directory", err, t)

		// wrong path: refresh with "bad token"
		t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", badTokenFile)
		_ = cs.populateFromEnv()
		err = cs.refreshFromService(context.Background())
		assertErr("STS HTTP request returned unexpected status: 401 Unauthorized", err, t)

		// happy path: refresh with "good token"
		t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", goodTokenFile)
		_ = cs.populateFromEnv()
		err = cs.refreshFromService(context.Background())
		if err != nil {
			t.Fatalf("Unexpected err: %s", err)
		}

		// happy path: refresh and get credentials
		creds, _ := cs.credentials(context.Background())
		assertEq(creds.AccessKey, testAccessKey, t)

		// happy path: refresh with session and get credentials
		cs.expiration = time.Now()
		cs.SessionName = "TEST_SESSION"
		creds, _ = cs.credentials(context.Background())
		assertEq(creds.AccessKey, testAccessKey, t)

		// happy path: don't refresh, but get credentials
		ts.accessKey = "OTHERKEY"
		creds, _ = cs.credentials(context.Background())
		assertEq(creds.AccessKey, testAccessKey, t)

		// happy/wrong path: refresh with "bad token" but return previous credentials
		t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", badTokenFile)
		_ = cs.populateFromEnv()
		cs.expiration = time.Now()
		creds, err = cs.credentials(context.Background())
		assertEq(creds.AccessKey, testAccessKey, t)
		assertErr("STS HTTP request returned unexpected status: 401 Unauthorized", err, t)

		// wrong path: refresh with "bad token" but return previous credentials
		t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", goodTokenFile)
		t.Setenv("AWS_ROLE_ARN", "BrokenRole")
		_ = cs.populateFromEnv()
		cs.expiration = time.Now()
		creds, err = cs.credentials(context.Background())
		assertErr("failed to parse credential response from STS service: EOF", err, t)
	})
}

func TestStsPath(t *testing.T) {
	cs := awsWebIdentityCredentialService{}

	defaultPath := fmt.Sprintf(stsDefaultPath, stsDefaultDomain)
	assertEq(defaultPath, cs.stsPath(), t)

	cs.RegionName = "us-east-2"
	assertEq("https://sts.us-east-2.amazonaws.com", cs.stsPath(), t)

	cs.Domain = "example.com"
	assertEq("https://sts.us-east-2.example.com", cs.stsPath(), t)

	cs.stsURL = "http://test.com"
	assertEq("http://test.com", cs.stsPath(), t)
}

func TestStsPathFromEnv(t *testing.T) {
	t.Setenv(awsRoleArnEnvVar, "role:arn")
	t.Setenv(awsWebIdentityTokenFileEnvVar, "/nonsense")

	tests := []struct {
		note string
		env  map[string]string
		cs   awsWebIdentityCredentialService
		want string
	}{
		{
			note: "region set in config",
			cs: awsWebIdentityCredentialService{
				RegionName: "us-east-2",
			},
			want: "https://sts.us-east-2.amazonaws.com",
		},
		{
			note: "region set in env",
			env: map[string]string{
				awsRegionEnvVar: "us-east-1",
			},
			want: "https://sts.us-east-1.amazonaws.com",
		},
		{
			note: "region set in env and config (config wins)",
			env: map[string]string{
				awsRegionEnvVar: "us-east-1",
			},
			cs: awsWebIdentityCredentialService{
				RegionName: "us-east-2",
			},
			want: "https://sts.us-east-2.amazonaws.com",
		},
		{
			note: "domain set in config",
			cs: awsWebIdentityCredentialService{
				RegionName: "us-east-2",
				Domain:     "foo.example.com",
			},
			want: "https://sts.us-east-2.foo.example.com",
		},
		{
			note: "domain set in env",
			env: map[string]string{
				awsDomainEnvVar: "bar.example.com",
			},
			cs: awsWebIdentityCredentialService{
				RegionName: "us-east-2", // Region must always be set
			},
			want: "https://sts.us-east-2.bar.example.com",
		},
		{
			note: "domain set in env and config (config wins)",
			env: map[string]string{
				awsDomainEnvVar: "bar.example.com",
			},
			cs: awsWebIdentityCredentialService{
				RegionName: "us-east-2", // Region must always be set
				Domain:     "foo.example.com",
			},
			want: "https://sts.us-east-2.foo.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if err := tc.cs.populateFromEnv(); err != nil {
				t.Fatalf("Unexpected err: %s", err)
			}
			assertEq(tc.want, tc.cs.stsPath(), t)
		})
	}
}

// simulate EC2 metadata service
type stsTestServer struct {
	t         *testing.T
	server    *httptest.Server
	accessKey string
}

func (t *stsTestServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || r.Method != http.MethodPost {
		w.WriteHeader(404)
		return
	}

	if err := r.ParseForm(); err != nil || r.PostForm.Get("Action") != "AssumeRoleWithWebIdentity" {
		w.WriteHeader(400)
		return
	}

	if r.PostForm.Get("RoleArn") == "BrokenRole" {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
		return
	}

	token := r.PostForm.Get("WebIdentityToken")
	if token != "good-token" {
		w.WriteHeader(401)
		return
	}
	w.WriteHeader(200)

	sessionName := r.PostForm.Get("RoleSessionName")

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

	_, _ = w.Write([]byte(fmt.Sprintf(xmlResponse, sessionName, time.Now().Add(time.Hour).Format(time.RFC3339), t.accessKey)))
}

func (t *stsTestServer) start() {
	t.server = httptest.NewServer(http.HandlerFunc(t.handle))
}

func (t *stsTestServer) stop() {
	t.server.Close()
}

func TestECRAuthPluginFailsWithoutAWSAuthPlugins(t *testing.T) {
	ap := newECRAuthPlugin(&awsSigningAuthPlugin{
		logger: logging.NewNoOpLogger(),
	})

	req := httptest.NewRequest("", "http://somewhere.com", nil)

	err := ap.Prepare(req)
	if err == nil {
		t.Error("ecrAuthPlugin.Prepare(): expected and error")
	}
}

func TestECRAuthPluginRequestsAuthorizationToken(t *testing.T) {
	// Environment credentials to sign the ecr get authorization token request
	t.Setenv(accessKeyEnvVar, "blablabla")
	t.Setenv(secretKeyEnvVar, "tatata")
	t.Setenv(awsRegionEnvVar, "us-east-1")
	t.Setenv(sessionTokenEnvVar, "lalala")

	awsAuthPlugin := awsSigningAuthPlugin{
		logger:                    logging.NewNoOpLogger(),
		AWSEnvironmentCredentials: &awsEnvironmentCredentialService{},
	}

	ap := newECRAuthPlugin(&awsAuthPlugin)
	ap.ecr = &ecrStub{token: aws.ECRAuthorizationToken{
		AuthorizationToken: "secret",
	}}

	req := httptest.NewRequest("", "http://somewhere.com", nil)

	if err := ap.Prepare(req); err != nil {
		t.Errorf("ecrAuthPlugin.Prepare() = %q", err)
	}

	got := req.Header.Get("Authorization")
	want := "Basic secret"
	if got != want {
		t.Errorf("req.Header.Get(\"Authorization\") = %q, want %q", got, want)
	}
}

type ecrStub struct {
	token aws.ECRAuthorizationToken
}

func (es *ecrStub) GetAuthorizationToken(context.Context, aws.Credentials, string) (aws.ECRAuthorizationToken, error) {
	return es.token, nil
}

func TestECRAuthPluginReusesCachedToken(t *testing.T) {
	logger := logging.NewNoOpLogger()
	ap := ecrAuthPlugin{
		token: aws.ECRAuthorizationToken{
			AuthorizationToken: "secret",
			ExpiresAt:          time.Now().Add(time.Hour),
		},
		awsAuthPlugin: &awsSigningAuthPlugin{
			logger: logger,
		},
		logger: logger,
	}

	req := httptest.NewRequest("", "http://somewhere.com", nil)

	if err := ap.Prepare(req); err != nil {
		t.Errorf("ecrAuthPlugin.Prepare() = %q", err)
	}

	got := req.Header.Get("Authorization")
	want := "Basic secret"
	if got != want {
		t.Errorf("req.Header.Get(\"Authorization\") = %q, want %q", got, want)
	}
}
