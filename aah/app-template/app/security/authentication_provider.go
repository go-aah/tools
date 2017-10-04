package security

import (
	"aahframework.org/aah.v0"
	"aahframework.org/config.v0"
	"aahframework.org/security.v0/authc"
)

var _ authc.Authenticator = (*AuthenticationProvider)(nil)

// AuthenticationProvider struct implements `authc.Authenticator` interface.
type AuthenticationProvider struct {
}

// Init method initializes the AuthenticationProvider, this method gets called
// during server start up.
func (a *AuthenticationProvider) Init(cfg *config.Config) error {

	// NOTE: Init is called on application startup

	return nil
}

// GetAuthenticationInfo method is `authc.Authenticator` interface
func (a *AuthenticationProvider) GetAuthenticationInfo(authcToken *authc.AuthenticationToken) (*authc.AuthenticationInfo, error) {

	//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
	// This code snippet provided as a reference
	//
	// Call your appropriate datasource here (such as DB, API, LDAP, etc)
	// to get the subject (aka user) authentication information.
	//
	// Form Auth Values from authcToken
	// 		authcToken.Identity => username
	// 		authcToken.Credential => passowrd
	//_____________________________________________________________________

	// user := models.FindUserByEmail(authcToken.Identity)
	// if user == nil {
	// 	// No subject exists, return nil and error
	// 	return nil, authc.ErrSubjectNotExists
	// }

	// User found, now create authentication info and return to the framework
	authcInfo := authc.NewAuthenticationInfo()
	// authcInfo.Principals = append(authcInfo.Principals,
	// 	&authc.Principal{
	// 		Value:     user.Email,
	// 		IsPrimary: true,
	// 		Realm:     "inmemory",
	// 	})
	// authcInfo.Credential = []byte(user.Password)
	// authcInfo.IsLocked = user.IsLocked
	// authcInfo.IsExpired = user.IsExpried

	return authcInfo, nil
}

func postAuthEvent(e *aah.Event) {
	ctx := e.Data.(*aah.Context)

	// Do post successful authentication actions...
	_ = ctx
}

func init() {
	aah.OnPostAuth(postAuthEvent)
}
