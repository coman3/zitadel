package oidc

import (
	"strings"
	"time"

	"github.com/caos/oidc/pkg/oidc"
	"github.com/caos/oidc/pkg/op"

	authreq_model "github.com/caos/zitadel/internal/auth_request/model"
	"github.com/caos/zitadel/internal/errors"
	"github.com/caos/zitadel/internal/project/model"
)

type Client struct {
	*model.ApplicationView
	defaultLoginURL            string
	defaultAccessTokenLifetime time.Duration
	defaultIdTokenLifetime     time.Duration
	allowedScopes              []string
}

func ClientFromBusiness(app *model.ApplicationView, defaultLoginURL string, defaultAccessTokenLifetime, defaultIdTokenLifetime time.Duration, allowedScopes []string) (op.Client, error) {
	if !app.IsOIDC {
		return nil, errors.ThrowInvalidArgument(nil, "OIDC-d5bhD", "client is not a proper oidc application")
	}
	return &Client{
			ApplicationView:            app,
			defaultLoginURL:            defaultLoginURL,
			defaultAccessTokenLifetime: defaultAccessTokenLifetime,
			defaultIdTokenLifetime:     defaultIdTokenLifetime,
			allowedScopes:              allowedScopes},
		nil
}

func (c *Client) ApplicationType() op.ApplicationType {
	return op.ApplicationType(c.OIDCApplicationType)
}

func (c *Client) AuthMethod() oidc.AuthMethod {
	return authMethodToOIDC(c.OIDCAuthMethodType)
}

func (c *Client) GetID() string {
	return c.OIDCClientID
}

func (c *Client) LoginURL(id string) string {
	return c.defaultLoginURL + id
}

func (c *Client) RedirectURIs() []string {
	return c.OIDCRedirectUris
}

func (c *Client) PostLogoutRedirectURIs() []string {
	return c.OIDCPostLogoutRedirectUris
}

func (c *Client) ResponseTypes() []oidc.ResponseType {
	return responseTypesToOIDC(c.OIDCResponseTypes)
}

func (c *Client) GrantTypes() []oidc.GrantType {
	return grantTypesToOIDC(c.OIDCGrantTypes)
}

func (c *Client) DevMode() bool {
	return c.ApplicationView.DevMode
}

func (c *Client) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string {
		if c.IDTokenRoleAssertion {
			return scopes
		}
		return removeScopeWithPrefix(scopes, ScopeProjectRolePrefix)
	}
}

func (c *Client) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string {
		if c.AccessTokenRoleAssertion {
			return scopes
		}
		return removeScopeWithPrefix(scopes, ScopeProjectRolePrefix)
	}
}

func (c *Client) AccessTokenLifetime() time.Duration {
	return c.defaultAccessTokenLifetime //PLANNED: impl from real client
}

func (c *Client) IDTokenLifetime() time.Duration {
	return c.defaultIdTokenLifetime //PLANNED: impl from real client
}

func (c *Client) AccessTokenType() op.AccessTokenType {
	return accessTokenTypeToOIDC(c.ApplicationView.AccessTokenType)
}

func (c *Client) IsScopeAllowed(scope string) bool {
	if strings.HasPrefix(scope, authreq_model.OrgDomainPrimaryScope) {
		return true
	}
	if strings.HasPrefix(scope, authreq_model.OrgIDScope) {
		return true
	}
	if strings.HasPrefix(scope, authreq_model.ProjectIDScope) {
		return true
	}
	if strings.HasPrefix(scope, ScopeUserMetaData) {
		return true
	}
	for _, allowedScope := range c.allowedScopes {
		if scope == allowedScope {
			return true
		}
	}
	return false
}

func (c *Client) ClockSkew() time.Duration {
	return c.ApplicationView.ClockSkew
}

func (c *Client) IDTokenUserinfoClaimsAssertion() bool {
	return c.ApplicationView.IDTokenUserinfoAssertion
}

func accessTokenTypeToOIDC(tokenType model.OIDCTokenType) op.AccessTokenType {
	switch tokenType {
	case model.OIDCTokenTypeBearer:
		return op.AccessTokenTypeBearer
	case model.OIDCTokenTypeJWT:
		return op.AccessTokenTypeJWT
	default:
		return op.AccessTokenTypeBearer
	}
}

func authMethodToOIDC(authType model.OIDCAuthMethodType) oidc.AuthMethod {
	switch authType {
	case model.OIDCAuthMethodTypeBasic:
		return oidc.AuthMethodBasic
	case model.OIDCAuthMethodTypePost:
		return oidc.AuthMethodPost
	case model.OIDCAuthMethodTypeNone:
		return oidc.AuthMethodNone
	case model.OIDCAuthMethodTypePrivateKeyJWT:
		return oidc.AuthMethodPrivateKeyJWT
	default:
		return oidc.AuthMethodBasic
	}
}

func responseTypesToOIDC(responseTypes []model.OIDCResponseType) []oidc.ResponseType {
	oidcTypes := make([]oidc.ResponseType, len(responseTypes))
	for i, t := range responseTypes {
		oidcTypes[i] = responseTypeToOIDC(t)
	}
	return oidcTypes
}

func responseTypeToOIDC(responseType model.OIDCResponseType) oidc.ResponseType {
	switch responseType {
	case model.OIDCResponseTypeCode:
		return oidc.ResponseTypeCode
	case model.OIDCResponseTypeIDTokenToken:
		return oidc.ResponseTypeIDToken
	case model.OIDCResponseTypeIDToken:
		return oidc.ResponseTypeIDTokenOnly
	default:
		return oidc.ResponseTypeCode
	}
}

func grantTypesToOIDC(grantTypes []model.OIDCGrantType) []oidc.GrantType {
	oidcTypes := make([]oidc.GrantType, len(grantTypes))
	for i, t := range grantTypes {
		oidcTypes[i] = grantTypeToOIDC(t)
	}
	return oidcTypes
}

func grantTypeToOIDC(grantType model.OIDCGrantType) oidc.GrantType {
	switch grantType {
	case model.OIDCGrantTypeAuthorizationCode:
		return oidc.GrantTypeCode
	case model.OIDCGrantTypeImplicit:
		return oidc.GrantTypeImplicit
	case model.OIDCGrantTypeRefreshToken:
		return oidc.GrantTypeRefreshToken
	default:
		return oidc.GrantTypeCode
	}
}

func removeScopeWithPrefix(scopes []string, scopePrefix ...string) []string {
	newScopeList := make([]string, 0)
	for _, scope := range scopes {
		hasPrefix := false
		for _, prefix := range scopePrefix {
			if strings.HasPrefix(scope, prefix) {
				hasPrefix = true
				continue
			}
		}
		if !hasPrefix {
			newScopeList = append(newScopeList, scope)
		}
	}
	return newScopeList
}
