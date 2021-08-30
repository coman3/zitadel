package domain

const (
	OrgDomainPrimaryScope = "urn:zitadel:iam:org:domain:primary:"
	OrgDomainPrimaryClaim = "urn:zitadel:iam:org:domain:primary"
	OrgIDScope            = "urn:zitadel:iam:org:id:"
	OrgIDClaim            = "urn:zitadel:iam:org:id"
	ProjectIDScope        = "urn:zitadel:iam:org:project:id:"
	AudSuffix             = ":aud"
)

//TODO: Change AuthRequest to interface and let oidcauthreqesut implement it
type Request interface {
	Type() AuthRequestType
	IsValid() bool
}

type AuthRequestType int32

const (
	AuthRequestTypeOIDC AuthRequestType = iota
	AuthRequestTypeSAML
)

type AuthRequestOIDC struct {
	Scopes        []string
	ResponseType  OIDCResponseType
	Nonce         string
	CodeChallenge *OIDCCodeChallenge
}

func (a *AuthRequestOIDC) Type() AuthRequestType {
	return AuthRequestTypeOIDC
}

func (a *AuthRequestOIDC) IsValid() bool {
	return len(a.Scopes) > 0 &&
		a.CodeChallenge == nil || a.CodeChallenge != nil && a.CodeChallenge.IsValid()
}

type AuthRequestSAML struct {
}

func (a *AuthRequestSAML) Type() AuthRequestType {
	return AuthRequestTypeSAML
}

func (a *AuthRequestSAML) IsValid() bool {
	return true
}
