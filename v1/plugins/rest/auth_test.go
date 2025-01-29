package rest

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/logging"
)

func TestOCIWithAWSAuthSetsUpECRAuthPlugin(t *testing.T) {
	conf := `{
		"type": "oci",
		"credentials": {
			"s3_signing": {
				"environment_credentials": {}
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	if _, err := client.config.Credentials.S3Signing.NewClient(client.config); err != nil {
		t.Fatalf("S3Signing.NewClient() = %q", err)
	}

	if client.config.Credentials.S3Signing.AWSService != "ecr" {
		t.Errorf("S3Signing.AWSService = %v, want = %v", client.config.Credentials.S3Signing.AWSService, "ecr")
	}

	if client.config.Credentials.S3Signing.ecrAuthPlugin == nil {
		t.Errorf("S3Signing.ecrAuthPlugin isn't setup")
	}
}

func TestOCIWithAWSWrongService(t *testing.T) {
	conf := `{
		"type": "oci",
		"credentials": {
			"s3_signing": {
				"service": "ec2",
				"environment_credentials": {}
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %q", err)
	}

	{
		_, err := client.config.Credentials.S3Signing.NewClient(client.config)
		if err == nil {
			t.Fatalf("S3Signing.NewClient(): expected error")
		}

		wantContains := "ec2"
		if !strings.Contains(err.Error(), wantContains) {
			t.Errorf("got: %q, should contain: %q", err.Error(), wantContains)
		}
	}
}

func TestECRWithoutOCIFails(t *testing.T) {
	conf := `{
		"credentials": {
			"s3_signing": {
				"service": "ecr",
				"environment_credentials": {}
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %q", err)
	}

	{
		_, err := client.config.Credentials.S3Signing.NewClient(client.config)
		if err == nil {
			t.Fatal("S3Signing.NewClient(): expected error")
		}

		wantContains := "oci"
		if !strings.Contains(err.Error(), wantContains) {
			t.Fatalf("S3Signing.NewClient() = %q, should contain = %q", err, wantContains)
		}
	}
}

func TestOauth2WithAWSKMS(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "http://localhost",
		"credentials": {
			"oauth2": {
				"grant_type": "client_credentials",
				"aws_kms": {
					"name": "arn:aws:kms:eu-west-1:account_no:key/key_id",
					"algorithm": "ECDSA_SHA_256"
				},
				"aws_signing": {
					"service": "kms",
					"environment_credentials": {
						"aws_default_region": "eu-west-1"
					}
				},
				"token_url": "https://localhost",
				"scopes": ["profile", "opa"],
				"additional_claims": {
					"aud": "some audience"
				}
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	if _, err := client.config.Credentials.OAuth2.NewClient(client.config); err != nil {
		t.Fatalf("OAuth2.NewClient() = %q", err)
	}

	if client.config.Credentials.OAuth2.AWSKmsKey.Name != "arn:aws:kms:eu-west-1:account_no:key/key_id" {
		t.Errorf("OAuth2.AWSKmsKey.Name = %v, want = %v", client.config.Credentials.OAuth2.AWSKmsKey.Name, "arn:aws:kms:eu-west-1:account_no:key/key_id")
	}

	if client.config.Credentials.OAuth2.AWSSigningPlugin.kmsSignPlugin == nil {
		t.Errorf("OAuth2.AWSSigningPlugin.kmsSignPlugin isn't setup")
	}
}

func TestAssumeRoleWithNoSigningProvider(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "https://my-example-opa-bucket.s3.eu-north-1.amazonaws.com",
		"credentials": {
			"s3_signing": {
				"service": "s3",
				"assume_role_credentials": {}
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.config.Credentials.S3Signing.NewClient(client.config)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	expErrMsg := "a AWS signing plugin must be specified when AssumeRole credential provider is enabled"
	if err.Error() != expErrMsg {
		t.Fatalf("expected error: %v but got: %v", expErrMsg, err)
	}
}

func TestAssumeRoleWithUnsupportedSigningProvider(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "https://my-example-opa-bucket.s3.eu-north-1.amazonaws.com",
		"credentials": {
			"s3_signing": {
				"service": "s3",
				"assume_role_credentials": {"aws_signing": {"web_identity_credentials": {}}}
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.config.Credentials.S3Signing.NewClient(client.config)
	if err == nil {
		t.Fatal("expected error but got nil")
	}

	expErrMsg := "unsupported AWS signing plugin with AssumeRole credential provider"
	if err.Error() != expErrMsg {
		t.Fatalf("expected error: %v but got: %v", expErrMsg, err)
	}
}

func TestOauth2WithClientAssertion(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "http://localhost",
		"credentials": {
			"oauth2": {
				"grant_type": "client_credentials",
				"token_url": "https://localhost",
				"scopes": ["profile", "opa"],
				"additional_claims": {
					"aud": "some audience"
				},
				"client_id": "123",
				"client_assertion": "abc123"
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	if _, err := client.config.Credentials.OAuth2.NewClient(client.config); err != nil {
		t.Fatalf("OAuth2.NewClient() = %q", err)
	}

	if client.config.Credentials.OAuth2.ClientAssertionType != defaultClientAssertionType {
		t.Errorf("OAuth2.ClientAssertionType = %v, want = %v", client.config.Credentials.OAuth2.ClientAssertionType, defaultClientAssertionType)
	}
}

func TestOauth2WithClientAssertionOverrideAssertionType(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "http://localhost",
		"credentials": {
			"oauth2": {
				"grant_type": "client_credentials",
				"token_url": "https://localhost",
				"scopes": ["profile", "opa"],
				"additional_claims": {
					"aud": "some audience"
				},
				"client_id": "123",
				"client_assertion": "abc123",
				"client_assertion_type": "urn:ietf:params:oauth:my-thing"
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	if _, err := client.config.Credentials.OAuth2.NewClient(client.config); err != nil {
		t.Fatalf("OAuth2.NewClient() = %q", err)
	}

	if client.config.Credentials.OAuth2.ClientAssertionType != "urn:ietf:params:oauth:my-thing" {
		t.Errorf("OAuth2.ClientAssertionType = %v, want = %v", client.config.Credentials.OAuth2.ClientAssertionType, "urn:ietf:params:oauth:my-thing")
	}
}

func TestOauth2WithClientAssertionPath(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "http://localhost",
		"credentials": {
			"oauth2": {
				"grant_type": "client_credentials",
				"token_url": "https://localhost",
				"scopes": ["profile", "opa"],
				"additional_claims": {
					"aud": "some audience"
				},
				"client_id": "123",
				"client_assertion_path": "/var/run/secrets/azure/tokens/azure-identity-token"
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	if _, err := client.config.Credentials.OAuth2.NewClient(client.config); err != nil {
		t.Fatalf("OAuth2.NewClient() = %q", err)
	}

	if client.config.Credentials.OAuth2.ClientAssertionType != defaultClientAssertionType {
		t.Errorf("OAuth2.ClientAssertionType = %v, want = %v", client.config.Credentials.OAuth2.ClientAssertionType, defaultClientAssertionType)
	}
}

func TestOauth2WithClientAssertionPathOverrideAssertionType(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "http://localhost",
		"credentials": {
			"oauth2": {
				"grant_type": "client_credentials",
				"token_url": "https://localhost",
				"scopes": ["profile", "opa"],
				"additional_claims": {
					"aud": "some audience"
				},
				"client_id": "123",
				"client_assertion_path": "/var/run/secrets/azure/tokens/azure-identity-token",
				"client_assertion_type": "urn:ietf:params:oauth:my-thing"
			}
		}
	}`

	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}

	if _, err := client.config.Credentials.OAuth2.NewClient(client.config); err != nil {
		t.Fatalf("OAuth2.NewClient() = %q", err)
	}

	if client.config.Credentials.OAuth2.ClientAssertionType != "urn:ietf:params:oauth:my-thing" {
		t.Errorf("OAuth2.ClientAssertionType = %v, want = %v", client.config.Credentials.OAuth2.ClientAssertionType, "urn:ietf:params:oauth:my-thing")
	}
}

func TestBearerTokenHeaderAttachement(t *testing.T) {
	conf := `{
		"name": "foo",
		"url": "http://localhost",
		"type":"oci",
		"credentials": {
	      "bearer": {
		    "token":"user:password",
          }, 
        }
    }`
	client, err := New([]byte(conf), map[string]*keys.Config{})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	var buf bytes.Buffer
	client.logger.SetLevel(logging.Debug)
	client.logger.(*logging.StandardLogger).SetOutput(&buf)

	_, err = client.config.Credentials.Bearer.NewClient(client.config)
	if err != nil {
		t.Fatalf("Bearer Auth Plugin new client should not error = %q", err)
	}

	err = client.config.Credentials.Bearer.Prepare(&http.Request{Response: &http.Response{StatusCode: http.StatusTemporaryRedirect}})
	if err != nil {
		t.Fatalf("Bearer Auth Plugin should not error on redirect = %q ", err)
	}
	if !strings.Contains(buf.String(), "not attaching authorization header as the response contains a redirect") {
		t.Fatalf("log debug output does not contain the message to confirm that the authorization header was not attached")
	}

	err = client.config.Credentials.Bearer.Prepare(&http.Request{Response: &http.Response{StatusCode: http.StatusTemporaryRedirect}})
	if err != nil {
		t.Fatalf("Bearer Auth Plugin should not error on redirect = %q ", err)
	}
	if !strings.Contains(buf.String(), "not attaching authorization header as the response contains a redirect") {
		t.Fatalf("log debug output does not contain the message to confirm that the authorization header was not attached")
	}

	err = client.config.Credentials.Bearer.Prepare(&http.Request{Header: http.Header{}})
	if err != nil {
		t.Fatalf("Bearer Auth Plugin should not error on redirect = %q ", err)
	}
	if !strings.Contains(buf.String(), "attaching authorization header") {
		t.Fatalf("log debug output should show that the authorization header is attached")
	}
}
