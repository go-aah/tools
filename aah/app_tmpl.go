package main

import (
	"text/template"

	"aahframework.org/essentials.v0"
)

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// appTmplData and its methods
//______________________________________________________________________________

const (
	typeWeb        = "web"
	typeAPI        = "api"
	typeWebSocket  = "websocket"
	storeCookie    = "cookie"
	storeFile      = "file"
	aahTmplExt     = ".atmpl"
	authForm       = "form"
	authBasic      = "basic"
	authGeneric    = "generic"
	authNone       = "none"
	basicFileRealm = "file-realm"
)

// appTmplData struct holds inputs collected from user for new aah creation
type appTmplData struct {
	Name                   string
	Type                   string
	ImportPath             string
	BaseDir                string
	ViewEngine             string
	ViewFileExt            string
	AuthScheme             string
	BasicAuthMode          string
	PasswordEncoderAlgo    string
	SessionStore           string
	SessionFileStorePath   string
	BasicAuthFileRealmPath string
	CORSEnable             bool
	TmplDemils             string
	SubTypes               []string
}

func (a *appTmplData) IsWebApp() bool {
	return a.Type == typeWeb
}

func (a *appTmplData) IsAPIApp() bool {
	return a.Type == typeAPI
}

func (a *appTmplData) IsWebSocketApp() bool {
	return a.Type == typeWebSocket
}

func (a *appTmplData) IsAuthSchemeForWeb() bool {
	return a.Type == typeWeb && (a.AuthScheme == authForm || a.AuthScheme == authBasic)
}

func (a *appTmplData) IsAuthSchemeForAPI() bool {
	return a.Type == typeAPI && (a.AuthScheme == authGeneric || a.AuthScheme == authBasic)
}

func (a *appTmplData) IsSecurityEnabled() bool {
	return !ess.IsStrEmpty(a.AuthScheme)
}

func (a *appTmplData) IsSubTypeAPI() bool {
	return a.checkSubType(typeAPI)
}

func (a *appTmplData) IsSubTypeWebSocket() bool {
	return a.checkSubType(typeWebSocket)
}

func (a *appTmplData) checkSubType(t string) bool {
	for _, v := range a.SubTypes {
		if v == t {
			return true
		}
	}
	return false
}

//‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾
// Template funcs
//______________________________________________________________________________

var appTemplateFuncs = template.FuncMap{
	"securerandomstring": func(length int) string {
		return ess.SecureRandomString(length)
	},
}
