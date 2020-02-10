package topdown

import (
	"encoding/json"
	"github.com/coreos/go-oidc"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestGetTrustedIdentityProviderManager(t *testing.T) {
	type args struct {
		trustedIdentityProviders []*string
	}

	googleIdP := "https://accounts.google.com"
	oktaIdP := "https://login.okta.com"
	notAnIdP := "https://wadfsfasdfdgweqwefwegwewrtgwetgiwefetgrbrhntunuimuimunifuniufhwerwvfghtujy6uyrfqwef.com"
	notSecureIdP := "http://accounts.google.com"

	allIdPs := []string{googleIdP, oktaIdP, notAnIdP, notSecureIdP}

	googleIdPVerifier, err := CreateOrGetVerifier(&googleIdP)
	if err != nil {
		t.Errorf("Failed to get idp verifier, %v", err)
	}

	oktaIdPVerifier, err := CreateOrGetVerifier(&oktaIdP)
	if err != nil {
		t.Errorf("Failed to get idp verifier, %v", err)
	}

	notAnIdPVerifier, err := CreateOrGetVerifier(&notAnIdP)
	if err == nil {
		t.Errorf("Failed to detect an invalid idp.")
	}

	notSecureIdPVerifier, err := CreateOrGetVerifier(&notSecureIdP)
	if err == nil {
		t.Errorf("Failed to detec another invalid idp.")
	}

	var OnlyGoogleIdP sync.Map
	OnlyGoogleIdP.Store(googleIdP, googleIdPVerifier)

	var OnlyOktaIdP sync.Map
	OnlyOktaIdP.Store(oktaIdP, oktaIdPVerifier)

	var OnlyNotAnIdP sync.Map
	OnlyNotAnIdP.Store(notAnIdP, notAnIdPVerifier)

	var OnlyNotSecureIdP sync.Map
	OnlyNotSecureIdP.Store(notSecureIdP, notSecureIdPVerifier)

	var GoogleAndOktaIdP sync.Map
	GoogleAndOktaIdP.Store(googleIdP, googleIdPVerifier)
	GoogleAndOktaIdP.Store(oktaIdP, oktaIdPVerifier)

	tests := []struct {
		name    string
		args    args
		want    *TrustedIdProviderManagerImpl
		wantErr bool
	}{
		{name: "Verify nil    returns empty", args: args{trustedIdentityProviders: nil},                             want: &TrustedIdProviderManagerImpl{trustedVerifiers: sync.Map{}},       wantErr: false},
		{name: "Verify 1      returns 1",     args: args{trustedIdentityProviders: []*string{&googleIdP}},           want: &TrustedIdProviderManagerImpl{trustedVerifiers: OnlyGoogleIdP},    wantErr: false},
		{name: "Verify new 1  returns 1",     args: args{trustedIdentityProviders: []*string{&oktaIdP}},             want: &TrustedIdProviderManagerImpl{trustedVerifiers: OnlyOktaIdP},      wantErr: false},
		{name: "Verify 2      returns 2",     args: args{trustedIdentityProviders: []*string{&oktaIdP, &googleIdP}}, want: &TrustedIdProviderManagerImpl{trustedVerifiers: GoogleAndOktaIdP}, wantErr: false},
		{name: "Verify 1 fake returns error", args: args{trustedIdentityProviders: []*string{&notAnIdP}},            want: &TrustedIdProviderManagerImpl{trustedVerifiers: OnlyNotAnIdP},     wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTrustedIdentityProviderManager(tt.args.trustedIdentityProviders)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTrustedIdentityProviderManager() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			for _, idp := range allIdPs {
				gotValue, _ := got.trustedVerifiers.Load(idp)
				wantedValue, _ := tt.want.trustedVerifiers.Load(idp)
				if gotValue != wantedValue {
					t.Errorf("GetTrustedIdentityProviderManager() got = %v, but want %v", gotValue, wantedValue)
				}
			}
		})
	}
}


type FetchTestTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType string `json:"token_type"`
}
func fetchLiveTestTokenFromAuth0TestAccount(t *testing.T) (*string, error){
	url := "https://dev-wa90c9xx.auth0.com/oauth/token"
	payload := strings.NewReader("{\"client_id\":\"0pGQ4pgCuPoH9dqFZnT7rWLL4dHU2YoS\",\"client_secret\":\"TQTFnCpbavunE5mCvivLwx0ONkphF7zh7dv5ECc_BFRyxCKhSQUIQLfFrHN_og3g\",\"audience\":\"http://opa-test-api-app/api\",\"grant_type\":\"client_credentials\"}")
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		t.Errorf("Testing account NewRequest broke, got error = %v", err)
		return nil, err
	}
	req.Header.Add("content-type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Errorf("Testing account DefaultClient.Do broke, got error = %v", err)
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Errorf("Testing account ReadAll broke, got error = %v", err)
		return nil, err
	}
	var resStruct FetchTestTokenResponse
	err = json.Unmarshal([]byte(body), &resStruct)
	if err != nil {
		t.Errorf("Testing account test-token fetching broke, got error = %v", err)
		return nil, err
	}
	return &resStruct.AccessToken, nil
}

func Test_trustedIdProviderManager_VerifyToken(t *testing.T) {

	//
	// Test IdP's and Verifiers
	//

	googleIdP := "https://accounts.google.com"
	googleIdPVerifier, err := CreateOrGetVerifier(&googleIdP)
	if err != nil {
		t.Errorf("Failed to get idp verifier, %v", err)
		t.FailNow()
	}
	var OnlyGoogleIdP sync.Map
	OnlyGoogleIdP.Store(googleIdP, googleIdPVerifier)


	TestLiveIdP := "https://dev-wa90c9xx.auth0.com/"
	TestLiveIdPVerifier, err := CreateOrGetVerifier(&TestLiveIdP)
	if err != nil {
		t.Errorf("Failed to get idp verifier, %v", err)
		t.FailNow()
	}
	var OnlyTestLiveIdP sync.Map
	OnlyTestLiveIdP.Store(TestLiveIdP, TestLiveIdPVerifier)

	//
	// Test Tokens
	//

	notValidToken := "hello world"

	// {
	// 	"alg": "RS256",
	// 	"kid": "1e9gdk7"
	// }
	// {
	// 	"iss": "http://server.example.com",
	// 	"sub": "248289761001",
	// 	"aud": "s6BhdRkqt3",
	// 	"nonce": "n-0S6_WzA2Mj",
	// 	"exp": 1311281970,
	// 	"iat": 1311280970
	// }
	validTokenFromUntrustedIdP := "eyJhbGciOiJSUzI1NiIsImtpZCI6IjFlOWdkazcifQ.ewogImlzcyI6ICJodHRwOi8vc2VydmVyLmV4YW1wbGUuY29tIiwKICJzdWIiOiAiMjQ4Mjg5NzYxMDAxIiwKICJhdWQiOiAiczZCaGRSa3F0MyIsCiAibm9uY2UiOiAibi0wUzZfV3pBMk1qIiwKICJleHAiOiAxMzExMjgxOTcwLAogImlhdCI6IDEzMTEyODA5NzAKfQ.ggW8hZ1EuVLuxNuuIJKX_V8a_OMXzR0EHR9R6jgdqrOOF4daGU96Sr_P6qJp6IcmD3HP99Obi1PRs-cwh3LO-p146waJ8IhehcwL7F09JdijmBqkvPeB2T9CJNqeGpe-gccMg4vfKjkM8FcGvnzZUN4_KSP0aAp1tOJ1zZwgjxqGByKHiOtX7TpdQyHE5lcMiKPXfEIQILVq0pc_E2DzL7emopWoaoZTF_m0_N0YzFC6g6EJbOEoRoSK5hoDalrcvRYLSrQAZZKflyuVCyixEoV9GfNQC3_osjzw2PAithfubEEBLuVVk4XUVrWOLrLl0nx7RkKU8NXNHq-rvKMzqg"

	// For testing only,
	// -----BEGIN PUBLIC KEY-----
	// 	MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQCEcy1q5GiRaVaWHrCFXZNeFseI
	// yHm/z796yiT+by9SSm42jvYJy6AqzXcxuDCiXDZcWsPsdXYiTCYQBxsJemAypLpC
	// dx+4ku4jU1T/4dwcj6YHno4ReB7WLbJGs6f/JGobQTtkiBZBCMT13R2gwKt29xub
	// tmxuL9pdGexiqTgGpQIDAQAB
	// -----END PUBLIC KEY-----
	// -----BEGIN RSA PRIVATE KEY-----
	// 	MIICXAIBAAKBgQCEcy1q5GiRaVaWHrCFXZNeFseIyHm/z796yiT+by9SSm42jvYJ
	// y6AqzXcxuDCiXDZcWsPsdXYiTCYQBxsJemAypLpCdx+4ku4jU1T/4dwcj6YHno4R
	// eB7WLbJGs6f/JGobQTtkiBZBCMT13R2gwKt29xubtmxuL9pdGexiqTgGpQIDAQAB
	// AoGADOmBkvsjapGfXFEvmkDOHg0QdLg+jkF9hEXyp09FiLsy1WTIfZn5SlLvfMxd
	// CWb98bDzirjExIEx8LwQmbLxb7i0x5AknFOHxYTdqDlJogR3a4O9DR8W14Z/UeP7
	// PhjK5zcapScrTt2VN7Ip6FwBIF4hgmlmmbDJJyYr1rPbYL0CQQD902vQUWiBIcYm
	// ApYZkkx9ve7uXLZKxbibTxTHNF3D20LKrAyJ93bftalhf16QX6YLk8e4d+U0HdVt
	// rMk18V2bAkEAhZWbrF5Xb8GA2lUN3dnjYRCv3JtDh+e8bmluIpRSzPv+KupesQyj
	// GP0i6otx/o0dCmN2wV7fzA+OJqjJBjSQvwJAfLz82+hV8jf119Inj7OM8bJ4jB11
	// 3HMkkPahIHCEr+69+TnqA5dgjPoKnoZoo4zN3hym5unM8vrCW16xl1fhhwJAfy8j
	// CWjFPNzyTm2ehzQVfewCVDrrf/DOAh2VQ40OjKX7p2Z/g3gxrPAOF1tuzFoUZTiv
	// 74nh8Ap7YClhQ+w2RwJBAPmMygTYKSGV9OyYLfdMeOuIDSTevqDgwqOMS6PkHGuq
	// 8jOerKerZe1weNcPdkUCz441WNstLtpcQQjLsjX8SmM=
	// -----END RSA PRIVATE KEY-----
	// {
	// 	"alg": "RS256",
	// 	"kid": "1e9gdk7"
	// }
	// {
	// 	"iss": "https://accounts.google.com",
	// 	"sub": "248289761001",
	// 	"aud": "s6BhdRkqt3",
	// 	"exp": 13112819700,
	// 	"iat": 1311280970
	// }
	badSignatureFromTrustedIdP := "eyJhbGciOiJSUzI1NiIsImtpZCI6IjFlOWdkazcifQ.eyJpc3MiOiJodHRwczovL2FjY291bnRzLmdvb2dsZS5jb20iLCJzdWIiOiIyNDgyODk3NjEwMDEiLCJhdWQiOiJzNkJoZFJrcXQzIiwiZXhwIjoxMzExMjgxOTcwMCwiaWF0IjoxMzExMjgwOTcwfQ.YymiWfg9XEzfJ8yYhyNX2KySQMYl8jUF0jESpcpQ1_mvIViMoOzsKBZK1yZqCTbCfFAuMsrlbmceO-g6hZmnQZPn27NSHz1X8ngBNSLZ0N23x-TYyniwVJRq64UauBZzOdGvmURxV3xPRbNiuWcgmZ1D5Jn8OMhd_2jzqakbhJM"

	validTestTokenFromTrustedIdP, err := fetchLiveTestTokenFromAuth0TestAccount(t)
	if err != nil {
		t.FailNow()
	}
	
	expectToken := oidc.IDToken{
		Issuer:          "https://dev-wa90c9xx.auth0.com/",
		Audience:        []string{"http://opa-test-api-app/api"},
	}

	//
	// Test case enumerations
	//

	type args struct {
		token *string
	}
	tests := []struct {
		name    string
		trust   *sync.Map
		args    args
		want    *oidc.IDToken
		wantErr bool
	}{
		{name: "Verify non-jwt token returns error", trust: &OnlyGoogleIdP, args: args{token: &notValidToken}, want: nil, wantErr: true},
		{name: "Verify token from untrusted issuer returns error", trust: &OnlyGoogleIdP, args: args{token: &validTokenFromUntrustedIdP}, want: nil, wantErr: true},
		{name: "Verify token with bad sig from trusted issuer returns error", trust: &OnlyGoogleIdP, args: args{token: &badSignatureFromTrustedIdP}, want: nil, wantErr: true},
		{name: "Verify token from trusted issuer returns claims", trust: &OnlyTestLiveIdP, args: args{token: validTestTokenFromTrustedIdP}, want: &expectToken, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idpm := &TrustedIdProviderManagerImpl{
				trustedVerifiers: *tt.trust,
			}
			got, err := idpm.VerifyToken(tt.args.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Bail out if the test was expecting an error.
			if err != nil {
				return
			}

			// Check a few of the claims are correct,
			if got.Issuer != tt.want.Issuer {
				t.Errorf("VerifyToken() got.Issuer = %v, wante %v", got, tt.want)
				return
			}
			if len(got.Audience) != len(tt.want.Audience) {
				t.Errorf("VerifyToken() got.Issuer = %v, wante %v", got, tt.want)
				return
			}
			if got.Audience[0] != tt.want.Audience[0] {
				t.Errorf("VerifyToken() got.Issuer = %v, wante %v", got, tt.want)
				return
			}
		})
	}
}