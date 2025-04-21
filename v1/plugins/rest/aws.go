// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rest

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/open-policy-agent/opa/internal/providers/aws"
	"github.com/open-policy-agent/opa/v1/logging"
)

const (
	// ref. https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	ec2DefaultCredServicePath = "http://169.254.169.254/latest/meta-data/iam/security-credentials/"

	// ref. https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html
	ec2DefaultTokenPath = "http://169.254.169.254/latest/api/token"

	// ref. https://docs.aws.amazon.com/AmazonECS/latest/userguide/task-iam-roles.html
	ecsDefaultCredServicePath       = "http://169.254.170.2"
	ecsRelativePathEnvVar           = "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"
	ecsFullPathEnvVar               = "AWS_CONTAINER_CREDENTIALS_FULL_URI"
	ecsAuthorizationTokenEnvVar     = "AWS_CONTAINER_AUTHORIZATION_TOKEN"
	ecsAuthorizationTokenFileEnvVar = "AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE"

	// ref. https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html
	stsDefaultDomain = "amazonaws.com"
	stsDefaultPath   = "https://sts.%s"
	stsRegionPath    = "https://sts.%s.%s"

	// ref. https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
	accessKeyEnvVar               = "AWS_ACCESS_KEY_ID"
	secretKeyEnvVar               = "AWS_SECRET_ACCESS_KEY"
	securityTokenEnvVar           = "AWS_SECURITY_TOKEN"
	sessionTokenEnvVar            = "AWS_SESSION_TOKEN"
	awsRegionEnvVar               = "AWS_REGION"
	awsDomainEnvVar               = "AWS_DOMAIN"
	awsRoleArnEnvVar              = "AWS_ROLE_ARN"
	awsWebIdentityTokenFileEnvVar = "AWS_WEB_IDENTITY_TOKEN_FILE"
	awsCredentialsFileEnvVar      = "AWS_SHARED_CREDENTIALS_FILE"
	awsConfigFileEnvVar           = "AWS_CONFIG_FILE"
	awsProfileEnvVar              = "AWS_PROFILE"

	// ref. https://docs.aws.amazon.com/sdkref/latest/guide/settings-global.html
	accessKeyGlobalSetting     = "aws_access_key_id"
	secretKeyGlobalSetting     = "aws_secret_access_key"
	securityTokenGlobalSetting = "aws_session_token"
)

// awsCredentialService represents the interface for AWS credential providers
type awsCredentialService interface {
	credentials(context.Context) (aws.Credentials, error)
}

// awsEnvironmentCredentialService represents an static environment-variable credential provider for AWS
type awsEnvironmentCredentialService struct {
	logger logging.Logger
}

func (*awsEnvironmentCredentialService) credentials(context.Context) (aws.Credentials, error) {
	var creds aws.Credentials
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
	// SessionToken is required if using temporary ENV credentials from assumed IAM role
	// Missing SessionToken results with 403 s3 error.
	creds.SessionToken = os.Getenv(sessionTokenEnvVar)
	if creds.SessionToken == "" {
		// In case of missing SessionToken try to get SecurityToken
		// AWS switched to use SessionToken, but SecurityToken was left for backward compatibility
		creds.SessionToken = os.Getenv(securityTokenEnvVar)
	}

	return creds, nil
}

type ssoSessionDetails struct {
	StartUrl              string `json:"startUrl"`
	Region                string `json:"region"`
	Name                  string
	AccountID             string
	RoleName              string
	AccessToken           string    `json:"accessToken"`
	ExpiresAt             time.Time `json:"expiresAt"`
	RegistrationExpiresAt time.Time `json:"registrationExpiresAt"`
	RefreshToken          string    `json:"refreshToken"`
	ClientId              string    `json:"clientId"`
	ClientSecret          string    `json:"clientSecret"`
}

type awsSSOCredentialsService struct {
	Path         string `json:"path,omitempty"`
	SSOCachePath string `json:"cache_path,omitempty"`

	Profile string `json:"profile,omitempty"`

	logger logging.Logger

	creds aws.Credentials

	credentialsExpiresAt time.Time

	session *ssoSessionDetails
}

func (cs *awsSSOCredentialsService) configPath() (string, error) {
	if len(cs.Path) != 0 {
		return cs.Path, nil
	}

	if cs.Path = os.Getenv(awsConfigFileEnvVar); len(cs.Path) != 0 {
		return cs.Path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home directory not found: %w", err)
	}

	cs.Path = filepath.Join(homeDir, ".aws", "config")

	return cs.Path, nil
}
func (cs *awsSSOCredentialsService) ssoCachePath() (string, error) {
	if len(cs.SSOCachePath) != 0 {
		return cs.SSOCachePath, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home directory not found: %w", err)
	}

	cs.Path = filepath.Join(homeDir, ".aws", "sso", "cache")

	return cs.Path, nil
}

func (cs *awsSSOCredentialsService) cacheKeyFileName() (string, error) {

	val := cs.session.StartUrl
	if cs.session.Name != "" {
		val = cs.session.Name
	}

	hash := sha1.New()
	hash.Write([]byte(val))
	cacheKey := hex.EncodeToString(hash.Sum(nil))

	return cacheKey + ".json", nil
}

func (cs *awsSSOCredentialsService) loadSSOCredentials() error {
	ssoCachePath, err := cs.ssoCachePath()
	if err != nil {
		return fmt.Errorf("failed to get sso cache path: %w", err)
	}

	cacheKeyFile, err := cs.cacheKeyFileName()
	if err != nil {
		return err
	}

	cacheFile := path.Join(ssoCachePath, cacheKeyFile)
	cache, err := os.ReadFile(cacheFile)
	if err != nil {
		return fmt.Errorf("failed to load cache file: %v", err)
	}

	if err := json.Unmarshal(cache, &cs.session); err != nil {
		return fmt.Errorf("failed to unmarshal cache file: %v", err)
	}

	return nil

}

func (cs *awsSSOCredentialsService) loadSession() error {
	configPath, err := cs.configPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}
	config, err := ini.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config file: %w", err)
	}

	section, err := config.GetSection("profile " + cs.Profile)

	if err != nil {
		return fmt.Errorf("failed to find profile %s", cs.Profile)
	}

	accountID, err := section.GetKey("sso_account_id")
	if err != nil {
		return fmt.Errorf("failed to find sso_account_id key in profile %s", cs.Profile)
	}

	region, err := section.GetKey("region")
	if err != nil {
		return fmt.Errorf("failed to find region key in profile %s", cs.Profile)
	}

	roleName, err := section.GetKey("sso_role_name")
	if err != nil {
		return fmt.Errorf("failed to find sso_role_name key in profile %s", cs.Profile)
	}

	ssoSession, err := section.GetKey("sso_session")
	if err != nil {
		return fmt.Errorf("failed to find sso_session key in profile %s", cs.Profile)
	}

	sessionName := ssoSession.Value()

	session, err := config.GetSection("sso-session " + sessionName)
	if err != nil {
		return fmt.Errorf("failed to find sso-session %s", sessionName)
	}

	startUrl, err := session.GetKey("sso_start_url")
	if err != nil {
		return fmt.Errorf("failed to find sso_start_url key in sso-session %s", sessionName)
	}

	cs.session = &ssoSessionDetails{
		StartUrl:  startUrl.Value(),
		Name:      sessionName,
		AccountID: accountID.Value(),
		Region:    region.Value(),
		RoleName:  roleName.Value(),
	}

	return nil
}

func (cs *awsSSOCredentialsService) tryRefreshToken() error {
	// Check if refresh token is empty
	if cs.session.RefreshToken == "" {
		return errors.New("refresh token is empty")
	}

	// Use the refresh token to get a new access token
	// using the clientId, clientSecret and refreshToken from the loaded token
	// return the new token
	// if error, return error

	type refreshTokenRequest struct {
		ClientId     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
		RefreshToken string `json:"refreshToken"`
		GrantType    string `json:"grantType"`
	}

	data := refreshTokenRequest{
		ClientId:     cs.session.ClientId,
		ClientSecret: cs.session.ClientSecret,
		RefreshToken: cs.session.RefreshToken,
		GrantType:    "refresh_token",
	}

	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh token request: %v", err)
	}

	endpoint := fmt.Sprintf("https://oidc.%s.amazonaws.com/token", cs.session.Region)
	r, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create new request: %v", err)
	}

	r.Header.Add("Content-Type", "application/json")
	c := &http.Client{}
	resp, err := c.Do(r)
	if err != nil {
		return fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()

	type refreshTokenResponse struct {
		AccessToken  string `json:"accessToken"`
		ExpiresIn    int    `json:"expiresIn"`
		RefreshToken string `json:"refreshToken"`
	}

	refreshedToken := refreshTokenResponse{}

	if err := json.NewDecoder(resp.Body).Decode(&refreshedToken); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	cs.session.AccessToken = refreshedToken.AccessToken
	cs.session.ExpiresAt = time.Now().Add(time.Duration(refreshedToken.ExpiresIn) * time.Second)
	cs.session.RefreshToken = refreshedToken.RefreshToken

	return nil
}

func (cs *awsSSOCredentialsService) refreshCredentials() error {
	url := fmt.Sprintf("https://portal.sso.%s.amazonaws.com/federation/credentials?account_id=%s&role_name=%s", cs.session.Region, cs.session.AccountID, cs.session.RoleName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+cs.session.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	type roleCredentials struct {
		AccessKeyId     string `json:"accessKeyId"`
		SecretAccessKey string `json:"secretAccessKey"`
		SessionToken    string `json:"sessionToken"`
		Expiration      int64  `json:"expiration"`
	}
	type getRoleCredentialsResponse struct {
		RoleCredentials roleCredentials `json:"roleCredentials"`
	}

	var result getRoleCredentialsResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	cs.creds = aws.Credentials{
		AccessKey:    result.RoleCredentials.AccessKeyId,
		SecretKey:    result.RoleCredentials.SecretAccessKey,
		SessionToken: result.RoleCredentials.SessionToken,
		RegionName:   cs.session.Region,
	}

	cs.credentialsExpiresAt = time.Unix(result.RoleCredentials.Expiration, 0)

	return nil
}

func (cs *awsSSOCredentialsService) loadProfile() {
	if cs.Profile != "" {
		return
	}

	cs.Profile = os.Getenv(awsProfileEnvVar)

	if cs.Profile == "" {
		cs.Profile = "default"
	}

}

func (cs *awsSSOCredentialsService) init() error {
	cs.loadProfile()

	if err := cs.loadSession(); err != nil {
		return fmt.Errorf("failed to load session: %w", err)
	}

	if err := cs.loadSSOCredentials(); err != nil {
		return fmt.Errorf("failed to load SSO credentials: %w", err)
	}

	// this enforces fetching credentials
	cs.credentialsExpiresAt = time.Unix(0, 0)
	return nil
}

func (cs *awsSSOCredentialsService) credentials(context.Context) (aws.Credentials, error) {
	if cs.session == nil {
		if err := cs.init(); err != nil {
			return aws.Credentials{}, err
		}
	}

	if cs.credentialsExpiresAt.Before(time.Now().Add(5 * time.Minute)) {
		// Check if the sso token we have is still valid,
		// if not, try to refresh it
		if cs.session.ExpiresAt.Before(time.Now()) {
			// we try and get a new token if we can
			if cs.session.RegistrationExpiresAt.Before(time.Now()) {
				return aws.Credentials{}, errors.New("cannot refresh token, registration expired")
			}

			if err := cs.tryRefreshToken(); err != nil {
				return aws.Credentials{}, fmt.Errorf("failed to refresh token: %w", err)
			}
		}

		if err := cs.refreshCredentials(); err != nil {
			return aws.Credentials{}, fmt.Errorf("failed to refresh credentials: %w", err)
		}
	}

	return cs.creds, nil
}

// awsProfileCredentialService represents a credential provider for AWS that extracts credentials from the AWS
// credentials file
type awsProfileCredentialService struct {

	// Path to the credentials file.
	//
	// If empty will look for "AWS_SHARED_CREDENTIALS_FILE" env variable. If the
	// env value is empty will default to current user's home directory.
	// Linux/OSX: "$HOME/.aws/credentials"
	// Windows:   "%USERPROFILE%\.aws\credentials"
	Path string `json:"path,omitempty"`

	// AWS Profile to extract credentials from the credentials file. If empty
	// will default to environment variable "AWS_PROFILE" or "default" if
	// environment variable is also not set.
	Profile string `json:"profile,omitempty"`

	RegionName string `json:"aws_region"`

	logger logging.Logger
}

func (cs *awsProfileCredentialService) credentials(context.Context) (aws.Credentials, error) {
	var creds aws.Credentials

	filename, err := cs.path()
	if err != nil {
		return creds, err
	}

	cfg, err := ini.Load(filename)
	if err != nil {
		return creds, fmt.Errorf("failed to read credentials file: %v", err)
	}

	profile, err := cfg.GetSection(cs.profile())
	if err != nil {
		return creds, fmt.Errorf("failed to get profile: %v", err)
	}

	creds.AccessKey = profile.Key(accessKeyGlobalSetting).String()
	if creds.AccessKey == "" {
		return creds, fmt.Errorf("profile \"%v\" in credentials file %v does not contain \"%v\"", cs.Profile, cs.Path, accessKeyGlobalSetting)
	}

	creds.SecretKey = profile.Key(secretKeyGlobalSetting).String()
	if creds.SecretKey == "" {
		return creds, fmt.Errorf("profile \"%v\" in credentials file %v does not contain \"%v\"", cs.Profile, cs.Path, secretKeyGlobalSetting)
	}

	creds.SessionToken = profile.Key(securityTokenGlobalSetting).String() // default to empty string

	if cs.RegionName == "" {
		if cs.RegionName = os.Getenv(awsRegionEnvVar); cs.RegionName == "" {
			return creds, errors.New("no " + awsRegionEnvVar + " set in environment or configuration")
		}
	}
	creds.RegionName = cs.RegionName

	return creds, nil
}

func (cs *awsProfileCredentialService) path() (string, error) {
	if len(cs.Path) != 0 {
		return cs.Path, nil
	}

	if cs.Path = os.Getenv(awsCredentialsFileEnvVar); len(cs.Path) != 0 {
		return cs.Path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home directory not found: %w", err)
	}

	cs.Path = filepath.Join(homeDir, ".aws", "credentials")

	return cs.Path, nil
}

func (cs *awsProfileCredentialService) profile() string {
	if cs.Profile != "" {
		return cs.Profile
	}

	cs.Profile = os.Getenv(awsProfileEnvVar)

	if cs.Profile == "" {
		cs.Profile = "default"
	}

	return cs.Profile
}

// awsMetadataCredentialService represents an EC2 metadata service credential provider for AWS
type awsMetadataCredentialService struct {
	RoleName        string `json:"iam_role,omitempty"`
	RegionName      string `json:"aws_region"`
	creds           aws.Credentials
	expiration      time.Time
	credServicePath string
	tokenPath       string
	logger          logging.Logger
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
	if isECS() {
		// first check if the relative env var exists; if so we use that otherwise we
		// use the "full" var
		if _, relativeExists := os.LookupEnv(ecsRelativePathEnvVar); relativeExists {
			return ecsDefaultCredServicePath + os.Getenv(ecsRelativePathEnvVar), nil
		}
		return os.Getenv(ecsFullPathEnvVar), nil
	}
	// if there's no role name and we don't appear to have a path to the
	// ECS container service, then the configuration is invalid
	return "", errors.New("metadata endpoint cannot be determined from settings and environment")
}

func (cs *awsMetadataCredentialService) tokenRequest(ctx context.Context) (*http.Request, error) {
	tokenURL := ec2DefaultTokenPath
	if cs.tokenPath != "" {
		// override for testing
		tokenURL = cs.tokenPath
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, tokenURL, nil)
	if err != nil {
		return nil, err
	}

	// we are going to use the token in the immediate future, so a long TTL is not necessary
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	return req, nil
}

func (cs *awsMetadataCredentialService) refreshFromService(ctx context.Context) error {
	// define the expected JSON payload from the EC2 credential service
	// ref. https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
	type metadataPayload struct {
		Code            string
		AccessKeyID     string `json:"AccessKeyId"`
		SecretAccessKey string
		Token           string
		Expiration      time.Time
	}

	// Short circuit if a reasonable amount of time until credential expiration remains
	const tokenExpirationMargin = 5 * time.Minute

	if time.Now().Add(tokenExpirationMargin).Before(cs.expiration) {
		cs.logger.Debug("Credentials previously obtained from metadata service still valid.")
		return nil
	}

	cs.logger.Debug("Obtaining credentials from metadata service.")
	metaDataURL, err := cs.urlForMetadataService()
	if err != nil {
		// configuration issue or missing ECS environment
		return err
	}

	// construct an HTTP client with a reasonably short timeout
	client := &http.Client{Timeout: time.Second * 10}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaDataURL, nil)
	if err != nil {
		return errors.New("unable to construct metadata HTTP request: " + err.Error())
	}

	// if using the AWS_CONTAINER_CREDENTIALS_FULL_URI variable, we need to associate the token
	// to the request
	if _, useFullPath := os.LookupEnv(ecsFullPathEnvVar); useFullPath {
		var token string
		tokenFilePath, tokenFilePathExists := os.LookupEnv(ecsAuthorizationTokenFileEnvVar)

		if tokenFilePathExists {
			tokenBytes, err := os.ReadFile(tokenFilePath)
			if err != nil {
				return errors.New("failed to read ECS metadata authorization token from file: " + err.Error())
			}
			token = string(tokenBytes)
			// If token doesn't exist as a file check if it exists as an environment variable
		} else {
			var tokenExists bool
			token, tokenExists = os.LookupEnv(ecsAuthorizationTokenEnvVar)
			if !tokenExists {
				return errors.New("unable to get ECS metadata authorization token")
			}
		}
		req.Header.Set("Authorization", token)
	}

	// if in the EC2 environment, we will use IMDSv2, which requires a session cookie from a
	// PUT request on the token endpoint before it will give the credentials, this provides
	// protection from SSRF attacks
	if !isECS() {
		tokenReq, err := cs.tokenRequest(ctx)
		if err != nil {
			return errors.New("unable to construct metadata token HTTP request: " + err.Error())
		}
		body, err := aws.DoRequestWithClient(tokenReq, client, "metadata token", cs.logger)
		if err != nil {
			return err
		}
		// token is the body of response; add to header of metadata request
		req.Header.Set("X-aws-ec2-metadata-token", string(body))
	}

	body, err := aws.DoRequestWithClient(req, client, "metadata", cs.logger)
	if err != nil {
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
	cs.creds.SessionToken = payload.Token
	cs.creds.RegionName = cs.RegionName

	return nil
}

func (cs *awsMetadataCredentialService) credentials(ctx context.Context) (aws.Credentials, error) {
	err := cs.refreshFromService(ctx)
	if err != nil {
		return cs.creds, err
	}
	return cs.creds, nil
}

// awsAssumeRoleCredentialService represents a STS credential service that uses active IAM credentials
// to obtain temporary security credentials generated by AWS STS via AssumeRole API operation
type awsAssumeRoleCredentialService struct {
	RegionName       string                `json:"aws_region"`
	RoleArn          string                `json:"iam_role_arn"`
	SessionName      string                `json:"session_name"`
	Domain           string                `json:"aws_domain"`
	AWSSigningPlugin *awsSigningAuthPlugin `json:"aws_signing,omitempty"`
	stsURL           string
	creds            aws.Credentials
	expiration       time.Time
	logger           logging.Logger
}

func (cs *awsAssumeRoleCredentialService) populateFromEnv() error {
	if cs.AWSSigningPlugin == nil {
		return errors.New("a AWS signing plugin must be specified when AssumeRole credential provider is enabled")
	}

	switch {
	case cs.AWSSigningPlugin.AWSEnvironmentCredentials != nil:
	case cs.AWSSigningPlugin.AWSProfileCredentials != nil:
	case cs.AWSSigningPlugin.AWSMetadataCredentials != nil:
	default:
		return errors.New("unsupported AWS signing plugin with AssumeRole credential provider")
	}

	if cs.AWSSigningPlugin.AWSMetadataCredentials != nil {
		if cs.AWSSigningPlugin.AWSMetadataCredentials.RegionName == "" {
			if cs.AWSSigningPlugin.AWSMetadataCredentials.RegionName = os.Getenv(awsRegionEnvVar); cs.AWSSigningPlugin.AWSMetadataCredentials.RegionName == "" {
				return errors.New("no " + awsRegionEnvVar + " set in environment or configuration")
			}
		}
	}

	if cs.AWSSigningPlugin.AWSSignatureVersion == "" {
		cs.AWSSigningPlugin.AWSSignatureVersion = "4"
	}

	if cs.Domain == "" {
		cs.Domain = os.Getenv(awsDomainEnvVar)
	}

	if cs.RegionName == "" {
		if cs.RegionName = os.Getenv(awsRegionEnvVar); cs.RegionName == "" {
			return errors.New("no " + awsRegionEnvVar + " set in environment or configuration")
		}
	}

	if cs.RoleArn == "" {
		if cs.RoleArn = os.Getenv(awsRoleArnEnvVar); cs.RoleArn == "" {
			return errors.New("no " + awsRoleArnEnvVar + " set in environment or configuration")
		}
	}

	return nil
}

func (cs *awsAssumeRoleCredentialService) signingCredentials(ctx context.Context) (aws.Credentials, error) {
	if cs.AWSSigningPlugin.AWSEnvironmentCredentials != nil {
		cs.AWSSigningPlugin.AWSEnvironmentCredentials.logger = cs.logger
		return cs.AWSSigningPlugin.AWSEnvironmentCredentials.credentials(ctx)
	}

	if cs.AWSSigningPlugin.AWSProfileCredentials != nil {
		cs.AWSSigningPlugin.AWSProfileCredentials.logger = cs.logger
		return cs.AWSSigningPlugin.AWSProfileCredentials.credentials(ctx)
	}

	cs.AWSSigningPlugin.AWSMetadataCredentials.logger = cs.logger
	return cs.AWSSigningPlugin.AWSMetadataCredentials.credentials(ctx)
}

func (cs *awsAssumeRoleCredentialService) stsPath() string {
	return getSTSPath(cs.Domain, cs.stsURL, cs.RegionName)
}

func (cs *awsAssumeRoleCredentialService) refreshFromService(ctx context.Context) error {
	// define the expected JSON payload from the EC2 credential service
	// ref. https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	type responsePayload struct {
		Result struct {
			Credentials struct {
				SessionToken    string
				SecretAccessKey string
				Expiration      time.Time
				AccessKeyID     string `xml:"AccessKeyId"`
			}
		} `xml:"AssumeRoleResult"`
	}

	// short circuit if a reasonable amount of time until credential expiration remains
	if time.Now().Add(time.Minute * 5).Before(cs.expiration) {
		cs.logger.Debug("Credentials previously obtained from sts service still valid.")
		return nil
	}

	cs.logger.Debug("Obtaining credentials from sts for role %s.", cs.RoleArn)

	var sessionName string
	if cs.SessionName == "" {
		sessionName = "open-policy-agent"
	} else {
		sessionName = cs.SessionName
	}

	queryVals := url.Values{
		"Action":          []string{"AssumeRole"},
		"RoleSessionName": []string{sessionName},
		"RoleArn":         []string{cs.RoleArn},
		"Version":         []string{"2011-06-15"},
	}
	stsRequestURL, _ := url.Parse(cs.stsPath())

	// construct an HTTP client with a reasonably short timeout
	client := &http.Client{Timeout: time.Second * 10}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stsRequestURL.String(), strings.NewReader(queryVals.Encode()))
	if err != nil {
		return errors.New("unable to construct STS HTTP request: " + err.Error())
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Note: Calls to AWS STS AssumeRole must be signed using the access key ID
	// and secret access key
	signingCreds, err := cs.signingCredentials(ctx)
	if err != nil {
		return err
	}

	err = aws.SignRequest(req, "sts", signingCreds, time.Now(), cs.AWSSigningPlugin.AWSSignatureVersion)
	if err != nil {
		return err
	}

	body, err := aws.DoRequestWithClient(req, client, "STS", cs.logger)
	if err != nil {
		return err
	}

	var payload responsePayload
	err = xml.Unmarshal(body, &payload)
	if err != nil {
		return errors.New("failed to parse credential response from STS service: " + err.Error())
	}

	cs.expiration = payload.Result.Credentials.Expiration
	cs.creds.AccessKey = payload.Result.Credentials.AccessKeyID
	cs.creds.SecretKey = payload.Result.Credentials.SecretAccessKey
	cs.creds.SessionToken = payload.Result.Credentials.SessionToken
	cs.creds.RegionName = cs.RegionName

	return nil
}

func (cs *awsAssumeRoleCredentialService) credentials(ctx context.Context) (aws.Credentials, error) {
	err := cs.refreshFromService(ctx)
	if err != nil {
		return cs.creds, err
	}
	return cs.creds, nil
}

// awsWebIdentityCredentialService represents an STS WebIdentity credential services
type awsWebIdentityCredentialService struct {
	RoleArn              string
	WebIdentityTokenFile string
	RegionName           string `json:"aws_region"`
	SessionName          string `json:"session_name"`
	Domain               string `json:"aws_domain"`
	stsURL               string
	creds                aws.Credentials
	expiration           time.Time
	logger               logging.Logger
}

func (cs *awsWebIdentityCredentialService) populateFromEnv() error {
	cs.RoleArn = os.Getenv(awsRoleArnEnvVar)
	if cs.RoleArn == "" {
		return errors.New("no " + awsRoleArnEnvVar + " set in environment")
	}
	cs.WebIdentityTokenFile = os.Getenv(awsWebIdentityTokenFileEnvVar)
	if cs.WebIdentityTokenFile == "" {
		return errors.New("no " + awsWebIdentityTokenFileEnvVar + " set in environment")
	}

	if cs.Domain == "" {
		cs.Domain = os.Getenv(awsDomainEnvVar)
	}

	if cs.RegionName == "" {
		if cs.RegionName = os.Getenv(awsRegionEnvVar); cs.RegionName == "" {
			return errors.New("no " + awsRegionEnvVar + " set in environment or configuration")
		}
	}
	return nil
}

func (cs *awsWebIdentityCredentialService) stsPath() string {
	return getSTSPath(cs.Domain, cs.stsURL, cs.RegionName)
}

func (cs *awsWebIdentityCredentialService) refreshFromService(ctx context.Context) error {
	// define the expected JSON payload from the EC2 credential service
	// ref. https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html
	type responsePayload struct {
		Result struct {
			Credentials struct {
				SessionToken    string
				SecretAccessKey string
				Expiration      time.Time
				AccessKeyID     string `xml:"AccessKeyId"`
			}
		} `xml:"AssumeRoleWithWebIdentityResult"`
	}

	// short circuit if a reasonable amount of time until credential expiration remains
	if time.Now().Add(time.Minute * 5).Before(cs.expiration) {
		cs.logger.Debug("Credentials previously obtained from sts service still valid.")
		return nil
	}

	cs.logger.Debug("Obtaining credentials from sts for role %s.", cs.RoleArn)

	var sessionName string
	if cs.SessionName == "" {
		sessionName = "open-policy-agent"
	} else {
		sessionName = cs.SessionName
	}

	tokenData, err := os.ReadFile(cs.WebIdentityTokenFile)
	if err != nil {
		return errors.New("unable to read web token for sts HTTP request: " + err.Error())
	}

	token := string(tokenData)

	queryVals := url.Values{
		"Action":           []string{"AssumeRoleWithWebIdentity"},
		"RoleSessionName":  []string{sessionName},
		"RoleArn":          []string{cs.RoleArn},
		"WebIdentityToken": []string{token},
		"Version":          []string{"2011-06-15"},
	}
	stsRequestURL, _ := url.Parse(cs.stsPath())

	// construct an HTTP client with a reasonably short timeout
	client := &http.Client{Timeout: time.Second * 10}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, stsRequestURL.String(), strings.NewReader(queryVals.Encode()))
	if err != nil {
		return errors.New("unable to construct STS HTTP request: " + err.Error())
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	body, err := aws.DoRequestWithClient(req, client, "STS", cs.logger)
	if err != nil {
		return err
	}

	var payload responsePayload
	err = xml.Unmarshal(body, &payload)
	if err != nil {
		return errors.New("failed to parse credential response from STS service: " + err.Error())
	}

	cs.expiration = payload.Result.Credentials.Expiration
	cs.creds.AccessKey = payload.Result.Credentials.AccessKeyID
	cs.creds.SecretKey = payload.Result.Credentials.SecretAccessKey
	cs.creds.SessionToken = payload.Result.Credentials.SessionToken
	cs.creds.RegionName = cs.RegionName

	return nil
}

func (cs *awsWebIdentityCredentialService) credentials(ctx context.Context) (aws.Credentials, error) {
	err := cs.refreshFromService(ctx)
	if err != nil {
		return cs.creds, err
	}
	return cs.creds, nil
}

func isECS() bool {
	// the special relative path URI is set by the container agent in the ECS environment only
	_, isECSRelative := os.LookupEnv(ecsRelativePathEnvVar)
	_, isECSFull := os.LookupEnv(ecsFullPathEnvVar)
	return isECSRelative || isECSFull
}

// ecrAuthPlugin authorizes requests to AWS ECR.
type ecrAuthPlugin struct {
	token aws.ECRAuthorizationToken

	// awsAuthPlugin is used to sign ecr authorization token requests.
	awsAuthPlugin *awsSigningAuthPlugin

	// ecr represents the service we request tokens from.
	ecr ecr

	logger logging.Logger
}

type ecr interface {
	GetAuthorizationToken(context.Context, aws.Credentials, string) (aws.ECRAuthorizationToken, error)
}

func newECRAuthPlugin(ap *awsSigningAuthPlugin) *ecrAuthPlugin {
	return &ecrAuthPlugin{
		awsAuthPlugin: ap,
		ecr:           aws.NewECR(ap.logger),
		logger:        ap.logger,
	}
}

// Prepare should be called with any request to AWS ECR.
// It takes care of retrieving an ECR authorization token to sign
// the request with.
func (ap *ecrAuthPlugin) Prepare(r *http.Request) error {
	if !ap.token.IsValid() {
		ap.logger.Debug("Refreshing ECR auth token")
		if err := ap.refreshAuthorizationToken(r.Context()); err != nil {
			return err
		}
	}

	ap.logger.Debug("Signing request with ECR authorization token")

	r.Header.Set("Authorization", "Basic "+ap.token.AuthorizationToken)
	return nil
}

func (ap *ecrAuthPlugin) refreshAuthorizationToken(ctx context.Context) error {
	creds, err := ap.awsAuthPlugin.awsCredentialService().credentials(ctx)
	if err != nil {
		return fmt.Errorf("failed to get aws credentials: %w", err)
	}

	token, err := ap.ecr.GetAuthorizationToken(ctx, creds, ap.awsAuthPlugin.AWSSignatureVersion)
	if err != nil {
		return fmt.Errorf("ecr: failed to get authorization token: %w", err)
	}

	ap.token = token
	return nil
}

// awsKMSSignPlugin signs digests using AWS KMS.
type awsKMSSignPlugin struct {

	// awsAuthPlugin is used to sign kms sign requests.
	awsAuthPlugin *awsSigningAuthPlugin

	// kms represents the service for signing digests.
	kms awskms

	logger logging.Logger
}

type awskms interface {
	SignDigest(ctx context.Context, digest []byte, keyID string, signingAlgorithm string, creds aws.Credentials, signatureVersion string) (string, error)
}

func newKMSSignPlugin(ap *awsSigningAuthPlugin) *awsKMSSignPlugin {
	return &awsKMSSignPlugin{
		awsAuthPlugin: ap,
		kms:           aws.NewKMS(ap.logger),
		logger:        ap.logger,
	}
}

func (ap *awsKMSSignPlugin) SignDigest(ctx context.Context, digest []byte, keyID string, signingAlgorithm string) (string, error) {
	creds, err := ap.awsAuthPlugin.awsCredentialService().credentials(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get aws credentials: %w", err)
	}

	signature, err := ap.kms.SignDigest(ctx, digest, keyID, signingAlgorithm, creds, ap.awsAuthPlugin.AWSSignatureVersion)
	if err != nil {
		return "", fmt.Errorf("kms: failed to sign digest: %w", err)
	}

	return signature, nil
}

func getSTSPath(stsDomain, stsURL, regionName string) string {
	var domain string
	if stsDomain != "" {
		domain = strings.ToLower(stsDomain)
	} else {
		domain = stsDefaultDomain
	}

	var stsPath string
	switch {
	case stsURL != "":
		stsPath = stsURL
	case regionName != "":
		stsPath = fmt.Sprintf(stsRegionPath, strings.ToLower(regionName), domain)
	default:
		stsPath = fmt.Sprintf(stsDefaultPath, domain)
	}
	return stsPath
}
