package rest

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/keys"
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
