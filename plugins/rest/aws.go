// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// ref. https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	ec2DefaultCredServicePath = "http://169.254.169.254/latest/meta-data/iam/security-credentials/"

	// ref. https://docs.aws.amazon.com/AmazonECS/latest/userguide/task-iam-roles.html
	ecsDefaultCredServicePath = "http://169.254.170.2"
	ecsRelativePathEnvVar     = "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"

	// ref. https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
	accessKeyEnvVar = "AWS_ACCESS_KEY_ID"
	secretKeyEnvVar = "AWS_SECRET_ACCESS_KEY"
	awsRegionEnvVar = "AWS_REGION"
)

// awsCredentials represents the credentials obtained from an AWS credential provider
type awsCredentials struct {
	AccessKey     string
	SecretKey     string
	RegionName    string
	SecurityToken string
}

// awsCredentialService represents the interface for AWS credential providers
type awsCredentialService interface {
	credentials() (awsCredentials, error)
}

// awsEnvironmentCredentialService represents an environment-variable credential provider for AWS
type awsEnvironmentCredentialService struct{}

func (cs *awsEnvironmentCredentialService) credentials() (awsCredentials, error) {
	var creds awsCredentials
	creds.AccessKey = os.Getenv(accessKeyEnvVar)
	if creds.AccessKey == "" {
		return creds, errors.New("no " + accessKeyEnvVar + " set in environment")
	}
	creds.SecretKey = os.Getenv(secretKeyEnvVar)
	if creds.SecretKey == "" {
		return creds, errors.New("no " + secretKeyEnvVar + " set in environment")
	}
	creds.RegionName = os.Getenv(awsRegionEnvVar)
	if creds.RegionName == "" {
		return creds, errors.New("no " + awsRegionEnvVar + " set in environment")
	}
	creds.SecurityToken = "" // not applicable to this credential provider
	return creds, nil
}

// awsMetadataCredentialService represents an EC2 metadata service credential provider for AWS
type awsMetadataCredentialService struct {
	RoleName        string `json:"iam_role,omitempty"`
	RegionName      string `json:"aws_region"`
	creds           awsCredentials
	expiration      time.Time
	credServicePath string
}

func (cs *awsMetadataCredentialService) urlForMetadataService() (string, error) {
	// override default path for testing
	if cs.credServicePath != "" {
		return cs.credServicePath + cs.RoleName, nil
	}
	// otherwise, normal flow
	// if a role name is provided, look up via the EC2 credential service
	if cs.RoleName != "" {
		return ec2DefaultCredServicePath + cs.RoleName, nil
	}
	// otherwise, check environment to see if it looks like we're in an ECS
	// container (with implied role association)
	ecsRelativePath, isECS := os.LookupEnv(ecsRelativePathEnvVar)
	if isECS {
		return ecsDefaultCredServicePath + ecsRelativePath, nil
	}
	// if there's no role name and we don't appear to have a path to the
	// ECS container service, then the configuration is invalid
	return "", errors.New("metadata endpoint cannot be determined from settings and environment")
}

func (cs *awsMetadataCredentialService) refreshFromService() error {
	// define the expected JSON payload from the EC2 credential service
	// ref. https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	type metadataPayload struct {
		Code            string
		AccessKeyID     string `json:"AccessKeyId"`
		SecretAccessKey string
		Token           string
		Expiration      time.Time
	}

	// short circuit if a reasonable amount of time until credential expiration remains
	if time.Now().Add(time.Minute * 5).Before(cs.expiration) {
		logrus.Debug("Credentials previously obtained from metadata service still valid.")
		return nil
	}

	logrus.Debug("Obtaining credentials from metadata service.")
	metaDataURL, err := cs.urlForMetadataService()
	if err != nil {
		// configuration issue or missing ECS environment
		return err
	}

	resp, err := http.Get(metaDataURL)
	if err != nil {
		// some kind of catastrophe talking to the EC2 metadata service
		return err
	}
	defer resp.Body.Close()

	logrus.WithFields(logrus.Fields{
		"url":     metaDataURL,
		"status":  resp.Status,
		"headers": resp.Header,
	}).Debug("Received response from metadata service.")

	if resp.StatusCode != 200 {
		// most probably a 404 due to a role that's not available; but cover all the bases
		return errors.New("metadata service HTTP request failed: " + resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		// deal with problems reading the body, whatever that might be
		return err
	}

	var payload metadataPayload
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return errors.New("failed to parse credential response from metadata service: " + err.Error())
	}

	// Only the EC2 endpoint returns the "Code" element which indicates whether the query was
	// successful; the ECS endpoint does not! Some other fields are missing in the ECS payload
	// but we do not depend on them.
	if cs.RoleName != "" && payload.Code != "Success" {
		return errors.New("metadata service query did not succeed: " + payload.Code)
	}

	cs.expiration = payload.Expiration
	cs.creds.AccessKey = payload.AccessKeyID
	cs.creds.SecretKey = payload.SecretAccessKey
	cs.creds.SecurityToken = payload.Token
	cs.creds.RegionName = cs.RegionName

	return nil
}

func (cs *awsMetadataCredentialService) credentials() (awsCredentials, error) {
	err := cs.refreshFromService()
	if err != nil {
		return cs.creds, err
	}
	return cs.creds, nil
}

func sha256MAC(message []byte, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return mac.Sum(nil)
}

func sortKeys(strMap map[string]string) []string {
	keys := make([]string, len(strMap))

	i := 0
	for k := range strMap {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

// signV4 modifies an http.Request to include an AWS V4 signature based on a credential provider
func signV4(req *http.Request, credService awsCredentialService, theTime time.Time) error {
	var body []byte
	if req.Body == nil {
		body = []byte("")
	} else {
		var err error
		body, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return errors.New("error getting request body: " + err.Error())
		}
	}
	creds, err := credService.credentials()
	if err != nil {
		return errors.New("error getting AWS credentials: " + err.Error())
	}

	bodyHexHash := fmt.Sprintf("%x", sha256.Sum256(body))

	now := theTime.UTC()

	// V4 signing has specific ideas of how it wants to see dates/times encoded
	dateNow := now.Format("20060102")
	iso8601Now := now.Format("20060102T150405Z")

	// certain mandatory headers for V4 signing
	awsHeaders := map[string]string{
		"host":                 req.URL.Hostname(),
		"x-amz-content-sha256": bodyHexHash,
		"x-amz-date":           iso8601Now}

	// the security token header is necessary for ephemeral credentials, e.g. from
	// the EC2 metadata service
	if creds.SecurityToken != "" {
		awsHeaders["x-amz-security-token"] = creds.SecurityToken
	}

	// ref. https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html

	// the "canonical request" is the normalized version of the AWS service access
	// that we're attempting to perform; in this case, a GET from an S3 bucket
	canonicalReq := req.Method + "\n"            // HTTP method
	canonicalReq += req.URL.EscapedPath() + "\n" // URI-escaped path
	canonicalReq += "\n"                         // query string; not implemented

	// include the values for the signed headers
	orderedKeys := sortKeys(awsHeaders)
	for _, k := range orderedKeys {
		canonicalReq += k + ":" + awsHeaders[k] + "\n"
	}
	canonicalReq += "\n" // linefeed to terminate headers

	// include the list of the signed headers
	headerList := strings.Join(orderedKeys, ";")
	canonicalReq += headerList + "\n"
	canonicalReq += bodyHexHash

	// the "string to sign" is a time-bounded, scoped request token which
	// is linked to the "canonical request" by inclusion of its SHA-256 hash
	strToSign := "AWS4-HMAC-SHA256\n"                                    // V4 signing with SHA-256 HMAC
	strToSign += iso8601Now + "\n"                                       // ISO 8601 time
	strToSign += dateNow + "/" + creds.RegionName + "/s3/aws4_request\n" // scoping for signature
	strToSign += fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalReq)))  // SHA-256 of canonical request

	// the "signing key" is generated by repeated HMAC-SHA256 based on the same
	// scoping that's included in the "string to sign"; but including the secret key
	// to allow AWS to validate it
	signingKey := sha256MAC([]byte(dateNow), []byte("AWS4"+creds.SecretKey))
	signingKey = sha256MAC([]byte(creds.RegionName), signingKey)
	signingKey = sha256MAC([]byte("s3"), signingKey)
	signingKey = sha256MAC([]byte("aws4_request"), signingKey)

	// the "signature" is finally the "string to sign" signed by the "signing key"
	signature := sha256MAC([]byte(strToSign), signingKey)

	// required format of Authorization header; n.b. the access key corresponding to
	// the secret key is included here
	authHdr := "AWS4-HMAC-SHA256 Credential=" + creds.AccessKey + "/" + dateNow
	authHdr += "/" + creds.RegionName + "/s3/aws4_request,"
	authHdr += "SignedHeaders=" + headerList + ","
	authHdr += "Signature=" + fmt.Sprintf("%x", signature)

	// add the computed Authorization
	req.Header.Add("Authorization", authHdr)

	// populate the other signed headers into the request
	for _, k := range orderedKeys {
		req.Header.Add(k, awsHeaders[k])
	}

	return nil
}
