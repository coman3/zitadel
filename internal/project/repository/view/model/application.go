package model

import (
	"encoding/json"
	"time"

	"github.com/caos/logging"
	"github.com/lib/pq"

	http_util "github.com/caos/zitadel/internal/api/http"
	"github.com/caos/zitadel/internal/domain"
	caos_errs "github.com/caos/zitadel/internal/errors"
	"github.com/caos/zitadel/internal/eventstore/v1/models"
	"github.com/caos/zitadel/internal/project/model"
	es_model "github.com/caos/zitadel/internal/project/repository/eventsourcing/model"
)

const (
	ApplicationKeyID            = "id"
	ApplicationKeyProjectID     = "project_id"
	ApplicationKeyResourceOwner = "resource_owner"
	ApplicationKeyOIDCClientID  = "oidc_client_id"
	ApplicationKeyName          = "app_name"
)

type ApplicationView struct {
	ID                             string                        `json:"appId" gorm:"column:id;primary_key"`
	ProjectID                      string                        `json:"-" gorm:"column:project_id"`
	Name                           string                        `json:"name" gorm:"column:app_name"`
	CreationDate                   time.Time                     `json:"-" gorm:"column:creation_date"`
	ChangeDate                     time.Time                     `json:"-" gorm:"column:change_date"`
	State                          int32                         `json:"-" gorm:"column:app_state"`
	ResourceOwner                  string                        `json:"-" gorm:"column:resource_owner"`
	ProjectRoleAssertion           bool                          `json:"projectRoleAssertion" gorm:"column:project_role_assertion"`
	ProjectRoleCheck               bool                          `json:"projectRoleCheck" gorm:"column:project_role_check"`
	HasProjectCheck                bool                          `json:"hasProjectCheck" gorm:"column:has_project_check"`
	RegisterOnProjectResourceOwner bool                          `json:"registerOnProjectResourceOwner" gorm:"column:register_on_project_resource_owner"`
	PrivateLabelingSetting         domain.PrivateLabelingSetting `json:"privateLabelingSetting" gorm:"column:private_labeling_setting"`
	LoginPolicySetting             domain.LoginPolicySetting     `json:"loginPolicySetting" gorm:"column:login_policy_setting"`

	IsOIDC                     bool           `json:"-" gorm:"column:is_oidc"`
	OIDCVersion                int32          `json:"oidcVersion" gorm:"column:oidc_version"`
	OIDCClientID               string         `json:"clientId" gorm:"column:oidc_client_id"`
	OIDCRedirectUris           pq.StringArray `json:"redirectUris" gorm:"column:oidc_redirect_uris"`
	OIDCResponseTypes          pq.Int64Array  `json:"responseTypes" gorm:"column:oidc_response_types"`
	OIDCGrantTypes             pq.Int64Array  `json:"grantTypes" gorm:"column:oidc_grant_types"`
	OIDCApplicationType        int32          `json:"applicationType" gorm:"column:oidc_application_type"`
	OIDCAuthMethodType         int32          `json:"authMethodType" gorm:"column:oidc_auth_method_type"`
	OIDCPostLogoutRedirectUris pq.StringArray `json:"postLogoutRedirectUris" gorm:"column:oidc_post_logout_redirect_uris"`
	NoneCompliant              bool           `json:"-" gorm:"column:none_compliant"`
	ComplianceProblems         pq.StringArray `json:"-" gorm:"column:compliance_problems"`
	DevMode                    bool           `json:"devMode" gorm:"column:dev_mode"`
	OriginAllowList            pq.StringArray `json:"-" gorm:"column:origin_allow_list"`
	AdditionalOrigins          pq.StringArray `json:"additionalOrigins" gorm:"column:additional_origins"`
	AccessTokenType            int32          `json:"accessTokenType" gorm:"column:access_token_type"`
	AccessTokenRoleAssertion   bool           `json:"accessTokenRoleAssertion" gorm:"column:access_token_role_assertion"`
	IDTokenRoleAssertion       bool           `json:"idTokenRoleAssertion" gorm:"column:id_token_role_assertion"`
	IDTokenUserinfoAssertion   bool           `json:"idTokenUserinfoAssertion" gorm:"column:id_token_userinfo_assertion"`
	ClockSkew                  time.Duration  `json:"clockSkew" gorm:"column:clock_skew"`

	Sequence uint64 `json:"-" gorm:"sequence"`
}

func ApplicationViewToModel(app *ApplicationView) *model.ApplicationView {
	return &model.ApplicationView{
		ID:                             app.ID,
		ProjectID:                      app.ProjectID,
		Name:                           app.Name,
		State:                          model.AppState(app.State),
		Sequence:                       app.Sequence,
		CreationDate:                   app.CreationDate,
		ChangeDate:                     app.ChangeDate,
		ResourceOwner:                  app.ResourceOwner,
		ProjectRoleAssertion:           app.ProjectRoleAssertion,
		ProjectRoleCheck:               app.ProjectRoleCheck,
		HasProjectCheck:                app.HasProjectCheck,
		RegisterOnProjectResourceOwner: app.RegisterOnProjectResourceOwner,
		PrivateLabelingSetting:         app.PrivateLabelingSetting,
		LoginPolicySetting:             app.LoginPolicySetting,

		IsOIDC:                     app.IsOIDC,
		OIDCVersion:                model.OIDCVersion(app.OIDCVersion),
		OIDCClientID:               app.OIDCClientID,
		OIDCRedirectUris:           app.OIDCRedirectUris,
		OIDCResponseTypes:          OIDCResponseTypesToModel(app.OIDCResponseTypes),
		OIDCGrantTypes:             OIDCGrantTypesToModel(app.OIDCGrantTypes),
		OIDCApplicationType:        model.OIDCApplicationType(app.OIDCApplicationType),
		OIDCAuthMethodType:         model.OIDCAuthMethodType(app.OIDCAuthMethodType),
		OIDCPostLogoutRedirectUris: app.OIDCPostLogoutRedirectUris,
		NoneCompliant:              app.NoneCompliant,
		ComplianceProblems:         app.ComplianceProblems,
		DevMode:                    app.DevMode,
		OriginAllowList:            app.OriginAllowList,
		AdditionalOrigins:          app.AdditionalOrigins,
		AccessTokenType:            model.OIDCTokenType(app.AccessTokenType),
		AccessTokenRoleAssertion:   app.AccessTokenRoleAssertion,
		IDTokenRoleAssertion:       app.IDTokenRoleAssertion,
		IDTokenUserinfoAssertion:   app.IDTokenUserinfoAssertion,
		ClockSkew:                  app.ClockSkew,
	}
}

func OIDCResponseTypesToModel(oidctypes []int64) []model.OIDCResponseType {
	result := make([]model.OIDCResponseType, len(oidctypes))
	for i, t := range oidctypes {
		result[i] = model.OIDCResponseType(t)
	}
	return result
}

func OIDCGrantTypesToModel(granttypes []int64) []model.OIDCGrantType {
	result := make([]model.OIDCGrantType, len(granttypes))
	for i, t := range granttypes {
		result[i] = model.OIDCGrantType(t)
	}
	return result
}

func ApplicationViewsToModel(roles []*ApplicationView) []*model.ApplicationView {
	result := make([]*model.ApplicationView, len(roles))
	for i, r := range roles {
		result[i] = ApplicationViewToModel(r)
	}
	return result
}

func (a *ApplicationView) AppendEventIfMyApp(event *models.Event) (err error) {
	view := new(ApplicationView)
	switch event.Type {
	case es_model.ApplicationAdded:
		err = view.SetData(event)
		if err != nil {
			return err
		}
	case es_model.ApplicationChanged,
		es_model.OIDCConfigAdded,
		es_model.OIDCConfigChanged,
		es_model.APIConfigAdded,
		es_model.APIConfigChanged,
		es_model.ApplicationDeactivated,
		es_model.ApplicationReactivated:
		err = view.SetData(event)
		if err != nil {
			return err
		}
	case es_model.ApplicationRemoved:
		err = view.SetData(event)
		if err != nil {
			return err
		}
	case es_model.ProjectChanged:
		return a.AppendEvent(event)
	case es_model.ProjectRemoved:
		return a.AppendEvent(event)
	default:
		return nil
	}
	if view.ID == a.ID {
		return a.AppendEvent(event)
	}
	return nil
}

func (a *ApplicationView) AppendEvent(event *models.Event) (err error) {
	a.Sequence = event.Sequence
	a.ChangeDate = event.CreationDate
	switch event.Type {
	case es_model.ApplicationAdded:
		a.setRootData(event)
		a.CreationDate = event.CreationDate
		a.ResourceOwner = event.ResourceOwner
		err = a.SetData(event)
	case es_model.OIDCConfigAdded:
		a.IsOIDC = true
		err = a.SetData(event)
		if err != nil {
			return err
		}
		a.setCompliance()
		return a.setOriginAllowList()
	case es_model.APIConfigAdded:
		a.IsOIDC = false
		return a.SetData(event)
	case es_model.ApplicationChanged:
		return a.SetData(event)
	case es_model.OIDCConfigChanged:
		err = a.SetData(event)
		if err != nil {
			return err
		}
		a.setCompliance()
		return a.setOriginAllowList()
	case es_model.APIConfigChanged:
		return a.SetData(event)
	case es_model.ProjectChanged:
		return a.setProjectChanges(event)
	case es_model.ApplicationDeactivated:
		a.State = int32(model.AppStateInactive)
	case es_model.ApplicationReactivated:
		a.State = int32(model.AppStateActive)
	case es_model.ApplicationRemoved, es_model.ProjectRemoved:
		a.State = int32(model.AppStateRemoved)
	}
	return err
}

func (a *ApplicationView) setRootData(event *models.Event) {
	a.ProjectID = event.AggregateID
}

func (a *ApplicationView) SetData(event *models.Event) error {
	if err := json.Unmarshal(event.Data, a); err != nil {
		logging.Log("EVEN-lo9ds").WithError(err).Error("could not unmarshal event data")
		return caos_errs.ThrowInternal(err, "MODEL-8suie", "Could not unmarshal data")
	}
	return nil
}

func (a *ApplicationView) setOriginAllowList() error {
	allowList := make([]string, 0)
	for _, redirect := range a.OIDCRedirectUris {
		origin, err := http_util.GetOriginFromURLString(redirect)
		if err != nil {
			return err
		}
		if !http_util.IsOriginAllowed(allowList, origin) {
			allowList = append(allowList, origin)
		}
	}
	for _, origin := range a.AdditionalOrigins {
		if !http_util.IsOriginAllowed(allowList, origin) {
			allowList = append(allowList, origin)
		}
	}
	a.OriginAllowList = allowList
	return nil
}

func (a *ApplicationView) setCompliance() {
	compliance := model.GetOIDCCompliance(model.OIDCVersion(a.OIDCVersion), model.OIDCApplicationType(a.OIDCApplicationType), OIDCGrantTypesToModel(a.OIDCGrantTypes), OIDCResponseTypesToModel(a.OIDCResponseTypes), model.OIDCAuthMethodType(a.OIDCAuthMethodType), a.OIDCRedirectUris)
	a.NoneCompliant = compliance.NoneCompliant
	a.ComplianceProblems = compliance.Problems
}

func (a *ApplicationView) setProjectChanges(event *models.Event) error {
	changes := struct {
		ProjectRoleAssertion           *bool                          `json:"projectRoleAssertion,omitempty"`
		ProjectRoleCheck               *bool                          `json:"projectRoleCheck,omitempty"`
		HasProjectCheck                *bool                          `json:"hasProjectCheck,omitempty"`
		RegisterOnProjectResourceOwner *bool                          `json:"registerOnProjectResourceOwner,omitempty"`
		PrivateLabelingSetting         *domain.PrivateLabelingSetting `json:"privateLabelingSetting,omitempty"`
		LoginPolicySetting             *domain.LoginPolicySetting     `json:"loginPolicySetting,omitempty"`
	}{}
	if err := json.Unmarshal(event.Data, &changes); err != nil {
		logging.Log("EVEN-DFbfg").WithError(err).Error("could not unmarshal event data")
		return caos_errs.ThrowInternal(err, "MODEL-Bw221", "Could not unmarshal data")
	}
	if changes.ProjectRoleAssertion != nil {
		a.ProjectRoleAssertion = *changes.ProjectRoleAssertion
	}
	if changes.ProjectRoleCheck != nil {
		a.ProjectRoleCheck = *changes.ProjectRoleCheck
	}
	if changes.HasProjectCheck != nil {
		a.HasProjectCheck = *changes.HasProjectCheck
	}
	if changes.RegisterOnProjectResourceOwner != nil {
		a.RegisterOnProjectResourceOwner = *changes.RegisterOnProjectResourceOwner
	}
	if changes.PrivateLabelingSetting != nil {
		a.PrivateLabelingSetting = *changes.PrivateLabelingSetting
	}
	if changes.LoginPolicySetting != nil {
		a.LoginPolicySetting = *changes.LoginPolicySetting
	}
	return nil
}
