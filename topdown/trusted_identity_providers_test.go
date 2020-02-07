package topdown

import (
	"github.com/coreos/go-oidc"
	"reflect"
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
	OnlyGoogleIdP.Store(&googleIdP, googleIdPVerifier)

	var OnlyOktaIdP sync.Map
	OnlyOktaIdP.Store(&oktaIdP, oktaIdPVerifier)

	var OnlyNotAnIdP sync.Map
	OnlyNotAnIdP.Store(&notAnIdP, notAnIdPVerifier)

	var OnlyNotSecureIdP sync.Map
	OnlyNotSecureIdP.Store(&notSecureIdP, notSecureIdPVerifier)

	var GoogleAndOktaIdP sync.Map
	GoogleAndOktaIdP.Store(&googleIdP, googleIdPVerifier)
	GoogleAndOktaIdP.Store(&oktaIdP, oktaIdPVerifier)

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

func Test_trustedIdProviderManager_VerifyToken(t *testing.T) {
	type fields struct {
		trustedVerifiers sync.Map
	}
	type args struct {
		token *string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *oidc.IDToken
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idpm := &TrustedIdProviderManagerImpl{
				trustedVerifiers: tt.fields.trustedVerifiers,
			}
			got, err := idpm.VerifyToken(tt.args.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("VerifyToken() got = %v, want %v", got, tt.want)
			}
		})
	}
}