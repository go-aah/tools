// Copyright (c) Jeevanandam M. (https://github.com/jeevatkm)
// Source code and usage is governed by a MIT style
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"text/template"
	"time"

	"aahframe.work/essentials"
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
	authOAuth2     = "oauth2"
	authGeneric    = "generic"
	authNone       = "none"
	basicFileRealm = "file-realm"
)

var domainNameKeyReplacer = strings.NewReplacer(" ", "_", "-", "_", ".", "_", "*", "_")

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
	TmplDelimLeft          string
	TmplDelimRight         string
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

func (a *appTmplData) DomainNameKey() string {
	return domainNameKeyReplacer.Replace(a.Name)
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

func (a *appTmplData) IsSessionConfigRequired() bool {
	return a.AuthScheme == authForm || a.AuthScheme == authOAuth2 || a.AuthScheme == authBasic
}

func (a *appTmplData) IsAuth(name string) bool {
	return strings.Contains(a.AuthScheme, name)
}

func (a *appTmplData) IsBasicAuthFileRealm() bool {
	return a.BasicAuthMode == basicFileRealm
}

func (a *appTmplData) CurrentYear() string {
	return time.Now().Format("2006")
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

var vreplace = strings.NewReplacer("_auth", "", "auth_", "")

var appTemplateFuncs = template.FuncMap{
	"securerandomstring": func(length int) string {
		return ess.SecureRandomString(length)
	},
	"variablename": func(v string) string {
		return toLowerCamelCase(vreplace.Replace(v))
	},
	"isauth": func(args map[string]interface{}, name string) bool {
		app := args["App"].(*appTmplData)
		return app.IsAuth(name)
	},
}
