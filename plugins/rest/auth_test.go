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
