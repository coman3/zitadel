package eventstore

import (
	"context"
	"time"

	"github.com/caos/logging"

	"github.com/caos/zitadel/internal/api/authz"
	"github.com/caos/zitadel/internal/auth/repository/eventsourcing/view"
	"github.com/caos/zitadel/internal/auth_request/model"
	cache "github.com/caos/zitadel/internal/auth_request/repository"
	"github.com/caos/zitadel/internal/command"
	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/errors"
	v1 "github.com/caos/zitadel/internal/eventstore/v1"
	es_models "github.com/caos/zitadel/internal/eventstore/v1/models"
	iam_model "github.com/caos/zitadel/internal/iam/model"
	iam_view_model "github.com/caos/zitadel/internal/iam/repository/view/model"
	"github.com/caos/zitadel/internal/id"
	org_model "github.com/caos/zitadel/internal/org/model"
	org_view_model "github.com/caos/zitadel/internal/org/repository/view/model"
	project_view_model "github.com/caos/zitadel/internal/project/repository/view/model"
	"github.com/caos/zitadel/internal/repository/iam"
	"github.com/caos/zitadel/internal/telemetry/tracing"
	user_model "github.com/caos/zitadel/internal/user/model"
	es_model "github.com/caos/zitadel/internal/user/repository/eventsourcing/model"
	user_view_model "github.com/caos/zitadel/internal/user/repository/view/model"
	grant_view_model "github.com/caos/zitadel/internal/usergrant/repository/view/model"
)

type AuthRequestRepo struct {
	Command      *command.Commands
	AuthRequests cache.AuthRequestCache
	View         *view.View
	Eventstore   v1.Eventstore

	UserSessionViewProvider   userSessionViewProvider
	UserViewProvider          userViewProvider
	UserCommandProvider       userCommandProvider
	UserEventProvider         userEventProvider
	OrgViewProvider           orgViewProvider
	LoginPolicyViewProvider   loginPolicyViewProvider
	LockoutPolicyViewProvider lockoutPolicyViewProvider
	IDPProviderViewProvider   idpProviderViewProvider
	UserGrantProvider         userGrantProvider
	ProjectProvider           projectProvider

	IdGenerator id.Generator

	PasswordCheckLifeTime      time.Duration
	ExternalLoginCheckLifeTime time.Duration
	MFAInitSkippedLifeTime     time.Duration
	SecondFactorCheckLifeTime  time.Duration
	MultiFactorCheckLifeTime   time.Duration

	IAMID string
}

type userSessionViewProvider interface {
	UserSessionByIDs(string, string) (*user_view_model.UserSessionView, error)
	UserSessionsByAgentID(string) ([]*user_view_model.UserSessionView, error)
	PrefixAvatarURL() string
}
type userViewProvider interface {
	UserByID(string) (*user_view_model.UserView, error)
	PrefixAvatarURL() string
}

type loginPolicyViewProvider interface {
	LoginPolicyByAggregateID(string) (*iam_view_model.LoginPolicyView, error)
}

type lockoutPolicyViewProvider interface {
	LockoutPolicyByAggregateID(string) (*iam_view_model.LockoutPolicyView, error)
}

type idpProviderViewProvider interface {
	IDPProvidersByAggregateIDAndState(string, iam_model.IDPConfigState) ([]*iam_view_model.IDPProviderView, error)
}

type userEventProvider interface {
	UserEventsByID(ctx context.Context, id string, sequence uint64) ([]*es_models.Event, error)
}

type userCommandProvider interface {
	BulkAddedHumanExternalIDP(ctx context.Context, userID, resourceOwner string, externalIDPs []*domain.ExternalIDP) error
}

type orgViewProvider interface {
	OrgByID(string) (*org_view_model.OrgView, error)
	OrgByPrimaryDomain(string) (*org_view_model.OrgView, error)
}

type userGrantProvider interface {
	ApplicationByClientID(context.Context, string) (*project_view_model.ApplicationView, error)
	UserGrantsByProjectAndUserID(string, string) ([]*grant_view_model.UserGrantView, error)
}

type projectProvider interface {
	ApplicationByClientID(context.Context, string) (*project_view_model.ApplicationView, error)
	OrgProjectMappingByIDs(orgID, projectID string) (*project_view_model.OrgProjectMapping, error)
}

func (repo *AuthRequestRepo) Health(ctx context.Context) error {
	return repo.AuthRequests.Health(ctx)
}

func (repo *AuthRequestRepo) CreateAuthRequest(ctx context.Context, request *domain.AuthRequest) (_ *domain.AuthRequest, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	reqID, err := repo.IdGenerator.Next()
	if err != nil {
		return nil, err
	}
	request.ID = reqID
	app, err := repo.View.ApplicationByClientID(ctx, request.ApplicationID)
	if err != nil {
		return nil, err
	}
	appIDs, err := repo.View.AppIDsFromProjectID(ctx, app.ProjectID)
	if err != nil {
		return nil, err
	}
	request.Audience = appIDs
	request.AppendAudIfNotExisting(app.ProjectID)
	request.ApplicationResourceOwner = app.ResourceOwner
	request.RegisterOnProjectResourceOwner = app.RegisterOnProjectResourceOwner
	request.PrivateLabelingSetting = app.PrivateLabelingSetting
	if err := setOrgID(repo.OrgViewProvider, request); err != nil {
		return nil, err
	}
	if request.LoginHint != "" {
		err = repo.checkLoginName(ctx, request, request.LoginHint)
		logging.LogWithFields("EVENT-aG311", "login name", request.LoginHint, "id", request.ID, "applicationID", request.ApplicationID, "traceID", tracing.TraceIDFromCtx(ctx)).OnError(err).Debug("login hint invalid")
	}
	err = repo.AuthRequests.SaveAuthRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (repo *AuthRequestRepo) AuthRequestByID(ctx context.Context, id, userAgentID string) (_ *domain.AuthRequest, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	return repo.getAuthRequestNextSteps(ctx, id, userAgentID, false)
}

func (repo *AuthRequestRepo) AuthRequestByIDCheckLoggedIn(ctx context.Context, id, userAgentID string) (_ *domain.AuthRequest, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	return repo.getAuthRequestNextSteps(ctx, id, userAgentID, true)
}

func (repo *AuthRequestRepo) SaveAuthCode(ctx context.Context, id, code, userAgentID string) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, id, userAgentID)
	if err != nil {
		return err
	}
	request.Code = code
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) AuthRequestByCode(ctx context.Context, code string) (_ *domain.AuthRequest, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.AuthRequests.GetAuthRequestByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	err = repo.fillPolicies(ctx, request)
	if err != nil {
		return nil, err
	}
	steps, err := repo.nextSteps(ctx, request, true)
	if err != nil {
		return nil, err
	}
	request.PossibleSteps = steps
	return request, nil
}

func (repo *AuthRequestRepo) DeleteAuthRequest(ctx context.Context, id string) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	return repo.AuthRequests.DeleteAuthRequest(ctx, id)
}

func (repo *AuthRequestRepo) CheckLoginName(ctx context.Context, id, loginName, userAgentID string) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, id, userAgentID)
	if err != nil {
		return err
	}
	err = repo.checkLoginName(ctx, request, loginName)
	if err != nil {
		return err
	}
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) SelectExternalIDP(ctx context.Context, authReqID, idpConfigID, userAgentID string) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, authReqID, userAgentID)
	if err != nil {
		return err
	}
	err = repo.checkSelectedExternalIDP(request, idpConfigID)
	if err != nil {
		return err
	}
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) CheckExternalUserLogin(ctx context.Context, authReqID, userAgentID string, externalUser *domain.ExternalUser, info *domain.BrowserInfo) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, authReqID, userAgentID)
	if err != nil {
		return err
	}
	err = repo.checkExternalUserLogin(ctx, request, externalUser.IDPConfigID, externalUser.ExternalUserID)
	if errors.IsNotFound(err) {
		if err := repo.setLinkingUser(ctx, request, externalUser); err != nil {
			return err
		}
		return err
	}
	if err != nil {
		return err
	}

	err = repo.Command.HumanExternalLoginChecked(ctx, request.UserOrgID, request.UserID, request.WithCurrentInfo(info))
	if err != nil {
		return err
	}
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) SetExternalUserLogin(ctx context.Context, authReqID, userAgentID string, externalUser *domain.ExternalUser) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, authReqID, userAgentID)
	if err != nil {
		return err
	}

	err = repo.setLinkingUser(ctx, request, externalUser)
	if err != nil {
		return err
	}
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) setLinkingUser(ctx context.Context, request *domain.AuthRequest, externalUser *domain.ExternalUser) error {
	request.LinkingUsers = append(request.LinkingUsers, externalUser)
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) SelectUser(ctx context.Context, id, userID, userAgentID string) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, id, userAgentID)
	if err != nil {
		return err
	}
	user, err := activeUserByID(ctx, repo.UserViewProvider, repo.UserEventProvider, repo.OrgViewProvider, repo.LockoutPolicyViewProvider, userID)
	if err != nil {
		return err
	}
	if request.RequestedOrgID != "" && request.RequestedOrgID != user.ResourceOwner {
		return errors.ThrowPreconditionFailed(nil, "EVENT-fJe2a", "Errors.User.NotAllowedOrg")
	}
	username := user.UserName
	if request.RequestedOrgID == "" {
		username = user.PreferredLoginName
	}
	request.SetUserInfo(user.ID, username, user.PreferredLoginName, user.DisplayName, user.AvatarKey, user.ResourceOwner)
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) VerifyPassword(ctx context.Context, id, userID, resourceOwner, password, userAgentID string, info *domain.BrowserInfo) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequestEnsureUser(ctx, id, userAgentID, userID)
	if err != nil {
		return err
	}
	policy, err := repo.getLockoutPolicy(ctx, resourceOwner)
	if err != nil {
		return err
	}
	return repo.Command.HumanCheckPassword(ctx, resourceOwner, userID, password, request.WithCurrentInfo(info), policy)
}

func (repo *AuthRequestRepo) VerifyMFAOTP(ctx context.Context, authRequestID, userID, resourceOwner, code, userAgentID string, info *domain.BrowserInfo) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequestEnsureUser(ctx, authRequestID, userAgentID, userID)
	if err != nil {
		return err
	}
	return repo.Command.HumanCheckMFAOTP(ctx, userID, code, resourceOwner, request.WithCurrentInfo(info))
}

func (repo *AuthRequestRepo) BeginMFAU2FLogin(ctx context.Context, userID, resourceOwner, authRequestID, userAgentID string) (login *domain.WebAuthNLogin, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()

	request, err := repo.getAuthRequestEnsureUser(ctx, authRequestID, userAgentID, userID)
	if err != nil {
		return nil, err
	}
	return repo.Command.HumanBeginU2FLogin(ctx, userID, resourceOwner, request, true)
}

func (repo *AuthRequestRepo) VerifyMFAU2F(ctx context.Context, userID, resourceOwner, authRequestID, userAgentID string, credentialData []byte, info *domain.BrowserInfo) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequestEnsureUser(ctx, authRequestID, userAgentID, userID)
	if err != nil {
		return err
	}
	return repo.Command.HumanFinishU2FLogin(ctx, userID, resourceOwner, credentialData, request, true)
}

func (repo *AuthRequestRepo) BeginPasswordlessSetup(ctx context.Context, userID, resourceOwner string) (login *domain.WebAuthNToken, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	return repo.Command.HumanAddPasswordlessSetup(ctx, userID, resourceOwner, true)
}

func (repo *AuthRequestRepo) VerifyPasswordlessSetup(ctx context.Context, userID, resourceOwner, userAgentID, tokenName string, credentialData []byte) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	_, err = repo.Command.HumanHumanPasswordlessSetup(ctx, userID, resourceOwner, tokenName, userAgentID, credentialData)
	return err
}

func (repo *AuthRequestRepo) BeginPasswordlessInitCodeSetup(ctx context.Context, userID, resourceOwner, codeID, verificationCode string) (login *domain.WebAuthNToken, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	return repo.Command.HumanAddPasswordlessSetupInitCode(ctx, userID, resourceOwner, codeID, verificationCode)
}

func (repo *AuthRequestRepo) VerifyPasswordlessInitCodeSetup(ctx context.Context, userID, resourceOwner, userAgentID, tokenName, codeID, verificationCode string, credentialData []byte) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	_, err = repo.Command.HumanPasswordlessSetupInitCode(ctx, userID, resourceOwner, tokenName, userAgentID, codeID, verificationCode, credentialData)
	return err
}

func (repo *AuthRequestRepo) BeginPasswordlessLogin(ctx context.Context, userID, resourceOwner, authRequestID, userAgentID string) (login *domain.WebAuthNLogin, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequestEnsureUser(ctx, authRequestID, userAgentID, userID)
	if err != nil {
		return nil, err
	}
	return repo.Command.HumanBeginPasswordlessLogin(ctx, userID, resourceOwner, request, true)
}

func (repo *AuthRequestRepo) VerifyPasswordless(ctx context.Context, userID, resourceOwner, authRequestID, userAgentID string, credentialData []byte, info *domain.BrowserInfo) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequestEnsureUser(ctx, authRequestID, userAgentID, userID)
	if err != nil {
		return err
	}
	return repo.Command.HumanFinishPasswordlessLogin(ctx, userID, resourceOwner, credentialData, request, true)
}

func (repo *AuthRequestRepo) LinkExternalUsers(ctx context.Context, authReqID, userAgentID string, info *domain.BrowserInfo) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, authReqID, userAgentID)
	if err != nil {
		return err
	}
	err = linkExternalIDPs(ctx, repo.UserCommandProvider, request)
	if err != nil {
		return err
	}
	err = repo.Command.HumanExternalLoginChecked(ctx, request.UserOrgID, request.UserID, request.WithCurrentInfo(info))
	if err != nil {
		return err
	}
	request.LinkingUsers = nil
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) ResetLinkingUsers(ctx context.Context, authReqID, userAgentID string) error {
	request, err := repo.getAuthRequest(ctx, authReqID, userAgentID)
	if err != nil {
		return err
	}
	request.LinkingUsers = nil
	request.SelectedIDPConfigID = ""
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) AutoRegisterExternalUser(ctx context.Context, registerUser *domain.Human, externalIDP *domain.ExternalIDP, orgMemberRoles []string, authReqID, userAgentID, resourceOwner string, info *domain.BrowserInfo) (err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()
	request, err := repo.getAuthRequest(ctx, authReqID, userAgentID)
	if err != nil {
		return err
	}
	human, err := repo.Command.RegisterHuman(ctx, resourceOwner, registerUser, externalIDP, orgMemberRoles)
	if err != nil {
		return err
	}
	request.UserID = human.AggregateID
	request.UserOrgID = human.ResourceOwner
	request.SelectedIDPConfigID = externalIDP.IDPConfigID
	request.LinkingUsers = nil
	err = repo.Command.HumanExternalLoginChecked(ctx, request.UserOrgID, request.UserID, request.WithCurrentInfo(info))
	if err != nil {
		return err
	}
	return repo.AuthRequests.UpdateAuthRequest(ctx, request)
}

func (repo *AuthRequestRepo) getAuthRequestNextSteps(ctx context.Context, id, userAgentID string, checkLoggedIn bool) (*domain.AuthRequest, error) {
	request, err := repo.getAuthRequest(ctx, id, userAgentID)
	if err != nil {
		return nil, err
	}
	steps, err := repo.nextSteps(ctx, request, checkLoggedIn)
	if err != nil {
		return nil, err
	}
	request.PossibleSteps = steps
	return request, nil
}

func (repo *AuthRequestRepo) getAuthRequestEnsureUser(ctx context.Context, authRequestID, userAgentID, userID string) (*domain.AuthRequest, error) {
	request, err := repo.getAuthRequest(ctx, authRequestID, userAgentID)
	if err != nil {
		return nil, err
	}
	if request.UserID != userID {
		return nil, errors.ThrowPreconditionFailed(nil, "EVENT-GBH32", "Errors.User.NotMatchingUserID")
	}
	_, err = activeUserByID(ctx, repo.UserViewProvider, repo.UserEventProvider, repo.OrgViewProvider, repo.LockoutPolicyViewProvider, request.UserID)
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (repo *AuthRequestRepo) getAuthRequest(ctx context.Context, id, userAgentID string) (*domain.AuthRequest, error) {
	request, err := repo.AuthRequests.GetAuthRequestByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if request.AgentID != userAgentID {
		return nil, errors.ThrowPermissionDenied(nil, "EVENT-adk13", "Errors.AuthRequest.UserAgentNotCorresponding")
	}
	err = repo.fillPolicies(ctx, request)
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (repo *AuthRequestRepo) getLoginPolicyAndIDPProviders(ctx context.Context, orgID string) (*domain.LoginPolicy, []*domain.IDPProvider, error) {
	policy, err := repo.getLoginPolicy(ctx, orgID)
	if err != nil {
		return nil, nil, err
	}
	if !policy.AllowExternalIDP {
		return policy.ToLoginPolicyDomain(), nil, nil
	}
	idpProviders, err := getLoginPolicyIDPProviders(repo.IDPProviderViewProvider, repo.IAMID, orgID, policy.Default)
	if err != nil {
		return nil, nil, err
	}

	providers := iam_model.IdpProviderViewsToDomain(idpProviders)
	return policy.ToLoginPolicyDomain(), providers, nil
}

func (repo *AuthRequestRepo) fillPolicies(ctx context.Context, request *domain.AuthRequest) error {
	orgID := request.RequestedOrgID
	if orgID == "" {
		orgID = request.UserOrgID
	}
	if orgID == "" {
		orgID = repo.IAMID
	}

	loginPolicy, idpProviders, err := repo.getLoginPolicyAndIDPProviders(ctx, orgID)
	if err != nil {
		return err
	}
	request.LoginPolicy = loginPolicy
	if idpProviders != nil {
		request.AllowedExternalIDPs = idpProviders
	}
	lockoutPolicy, err := repo.getLockoutPolicy(ctx, orgID)
	if err != nil {
		return err
	}
	request.LockoutPolicy = lockoutPolicy
	privacyPolicy, err := repo.getPrivacyPolicy(ctx, orgID)
	if err != nil {
		return err
	}
	request.PrivacyPolicy = privacyPolicy
	privateLabelingOrgID := domain.IAMID
	if request.PrivateLabelingSetting != domain.PrivateLabelingSettingUnspecified {
		privateLabelingOrgID = request.ApplicationResourceOwner
	}
	if request.PrivateLabelingSetting == domain.PrivateLabelingSettingAllowLoginUserResourceOwnerPolicy || request.PrivateLabelingSetting == domain.PrivateLabelingSettingUnspecified {
		if request.UserOrgID != "" {
			privateLabelingOrgID = request.UserOrgID
		}
	}
	labelPolicy, err := repo.getLabelPolicy(ctx, privateLabelingOrgID)
	if err != nil {
		return err
	}
	request.LabelPolicy = labelPolicy
	defaultLoginTranslations, err := repo.getLoginTexts(ctx, domain.IAMID)
	if err != nil {
		return err
	}
	request.DefaultTranslations = defaultLoginTranslations
	orgLoginTranslations, err := repo.getLoginTexts(ctx, orgID)
	if err != nil {
		return err
	}
	request.OrgTranslations = orgLoginTranslations
	return nil
}

func (repo *AuthRequestRepo) checkLoginName(ctx context.Context, request *domain.AuthRequest, loginName string) (err error) {
	user := new(user_view_model.UserView)
	if request.RequestedOrgID != "" {
		preferredLoginName := loginName
		if request.RequestedOrgID != "" {
			preferredLoginName += "@" + request.RequestedPrimaryDomain
		}
		user, err = repo.View.UserByLoginNameAndResourceOwner(preferredLoginName, request.RequestedOrgID)
	} else {
		user, err = repo.View.UserByLoginName(loginName)
		if err == nil {
			err = repo.checkLoginPolicyWithResourceOwner(ctx, request, user)
			if err != nil {
				return err
			}
		}
	}
	if err != nil {
		return err
	}
	if user.State == int32(domain.UserStateInactive) {
		return errors.ThrowPreconditionFailed(nil, "AUTH-2n8fs", "Errors.User.Inactive")
	}
	request.SetUserInfo(user.ID, loginName, user.PreferredLoginName, "", "", user.ResourceOwner)
	return nil
}

func (repo AuthRequestRepo) checkLoginPolicyWithResourceOwner(ctx context.Context, request *domain.AuthRequest, user *user_view_model.UserView) error {
	loginPolicy, idpProviders, err := repo.getLoginPolicyAndIDPProviders(ctx, user.ResourceOwner)
	if err != nil {
		return err
	}
	if len(request.LinkingUsers) != 0 && !loginPolicy.AllowExternalIDP {
		return errors.ThrowInvalidArgument(nil, "LOGIN-s9sio", "Errors.User.NotAllowedToLink")
	}
	if len(request.LinkingUsers) != 0 {
		exists := linkingIDPConfigExistingInAllowedIDPs(request.LinkingUsers, idpProviders)
		if !exists {
			return errors.ThrowInvalidArgument(nil, "LOGIN-Dj89o", "Errors.User.NotAllowedToLink")
		}
	}
	request.LoginPolicy = loginPolicy
	request.AllowedExternalIDPs = idpProviders
	return nil
}

func (repo *AuthRequestRepo) checkSelectedExternalIDP(request *domain.AuthRequest, idpConfigID string) error {
	for _, externalIDP := range request.AllowedExternalIDPs {
		if externalIDP.IDPConfigID == idpConfigID {
			request.SelectedIDPConfigID = idpConfigID
			return nil
		}
	}
	return errors.ThrowNotFound(nil, "LOGIN-Nsm8r", "Errors.User.ExternalIDP.NotAllowed")
}

func (repo *AuthRequestRepo) checkExternalUserLogin(ctx context.Context, request *domain.AuthRequest, idpConfigID, externalUserID string) (err error) {
	externalIDP := new(user_view_model.ExternalIDPView)
	if request.RequestedOrgID != "" {
		externalIDP, err = repo.View.ExternalIDPByExternalUserIDAndIDPConfigIDAndResourceOwner(externalUserID, idpConfigID, request.RequestedOrgID)
	} else {
		externalIDP, err = repo.View.ExternalIDPByExternalUserIDAndIDPConfigID(externalUserID, idpConfigID)
	}
	if err != nil {
		return err
	}
	user, err := activeUserByID(ctx, repo.UserViewProvider, repo.UserEventProvider, repo.OrgViewProvider, repo.LockoutPolicyViewProvider, externalIDP.UserID)
	if err != nil {
		return err
	}
	username := user.UserName
	if request.RequestedOrgID == "" {
		username = user.PreferredLoginName
	}
	request.SetUserInfo(user.ID, username, user.PreferredLoginName, user.DisplayName, user.AvatarKey, user.ResourceOwner)
	return nil
}

func (repo *AuthRequestRepo) nextSteps(ctx context.Context, request *domain.AuthRequest, checkLoggedIn bool) ([]domain.NextStep, error) {
	if request == nil {
		return nil, errors.ThrowInvalidArgument(nil, "EVENT-ds27a", "Errors.Internal")
	}
	steps := make([]domain.NextStep, 0)
	if !checkLoggedIn && domain.IsPrompt(request.Prompt, domain.PromptNone) {
		return append(steps, &domain.RedirectToCallbackStep{}), nil
	}
	if request.UserID == "" {
		if request.LinkingUsers != nil && len(request.LinkingUsers) > 0 {
			steps = append(steps, new(domain.ExternalNotFoundOptionStep))
			return steps, nil
		}
		steps = append(steps, new(domain.LoginStep))
		if domain.IsPrompt(request.Prompt, domain.PromptCreate) {
			return append(steps, &domain.RegistrationStep{}), nil
		}
		if len(request.Prompt) == 0 || domain.IsPrompt(request.Prompt, domain.PromptSelectAccount) {
			users, err := repo.usersForUserSelection(request)
			if err != nil {
				return nil, err
			}
			if len(users) > 0 || domain.IsPrompt(request.Prompt, domain.PromptSelectAccount) {
				steps = append(steps, &domain.SelectUserStep{Users: users})
			}
		}
		return steps, nil
	}
	user, err := activeUserByID(ctx, repo.UserViewProvider, repo.UserEventProvider, repo.OrgViewProvider, repo.LockoutPolicyViewProvider, request.UserID)
	if err != nil {
		return nil, err
	}
	request.LoginName = user.PreferredLoginName
	userSession, err := userSessionByIDs(ctx, repo.UserSessionViewProvider, repo.UserEventProvider, request.AgentID, user)
	if err != nil {
		return nil, err
	}

	isInternalLogin := request.SelectedIDPConfigID == "" && userSession.SelectedIDPConfigID == ""
	if !isInternalLogin && len(request.LinkingUsers) == 0 && !checkVerificationTimeMaxAge(userSession.ExternalLoginVerification, repo.ExternalLoginCheckLifeTime, request) {
		selectedIDPConfigID := request.SelectedIDPConfigID
		if selectedIDPConfigID == "" {
			selectedIDPConfigID = userSession.SelectedIDPConfigID
		}
		return append(steps, &domain.ExternalLoginStep{SelectedIDPConfigID: selectedIDPConfigID}), nil
	}
	if isInternalLogin || (!isInternalLogin && len(request.LinkingUsers) > 0) {
		step := repo.firstFactorChecked(request, user, userSession)
		if step != nil {
			return append(steps, step), nil
		}
	}

	step, ok, err := repo.mfaChecked(userSession, request, user)
	if err != nil {
		return nil, err
	}
	if !ok {
		return append(steps, step), nil
	}

	if user.PasswordChangeRequired {
		steps = append(steps, &domain.ChangePasswordStep{})
	}
	if !user.IsEmailVerified {
		steps = append(steps, &domain.VerifyEMailStep{})
	}
	if user.UsernameChangeRequired {
		steps = append(steps, &domain.ChangeUsernameStep{})
	}

	if user.PasswordChangeRequired || !user.IsEmailVerified || user.UsernameChangeRequired {
		return steps, nil
	}

	if request.LinkingUsers != nil && len(request.LinkingUsers) != 0 {
		return append(steps, &domain.LinkUsersStep{}), nil
	}
	//PLANNED: consent step

	missing, err := projectRequired(ctx, request, repo.ProjectProvider)
	if err != nil {
		return nil, err
	}
	if missing {
		return append(steps, &domain.ProjectRequiredStep{}), nil
	}

	missing, err = userGrantRequired(ctx, request, user, repo.UserGrantProvider)
	if err != nil {
		return nil, err
	}
	if missing {
		return append(steps, &domain.GrantRequiredStep{}), nil
	}

	return append(steps, &domain.RedirectToCallbackStep{}), nil
}

func (repo *AuthRequestRepo) usersForUserSelection(request *domain.AuthRequest) ([]domain.UserSelection, error) {
	userSessions, err := userSessionsByUserAgentID(repo.UserSessionViewProvider, request.AgentID)
	if err != nil {
		return nil, err
	}
	users := make([]domain.UserSelection, len(userSessions))
	for i, session := range userSessions {
		users[i] = domain.UserSelection{
			UserID:            session.UserID,
			DisplayName:       session.DisplayName,
			UserName:          session.UserName,
			LoginName:         session.LoginName,
			ResourceOwner:     session.ResourceOwner,
			AvatarKey:         session.AvatarKey,
			UserSessionState:  model.UserSessionStateToDomain(session.State),
			SelectionPossible: request.RequestedOrgID == "" || request.RequestedOrgID == session.ResourceOwner,
		}
	}
	return users, nil
}

func (repo *AuthRequestRepo) firstFactorChecked(request *domain.AuthRequest, user *user_model.UserView, userSession *user_model.UserSessionView) domain.NextStep {
	if user.InitRequired {
		return &domain.InitUserStep{PasswordSet: user.PasswordSet}
	}

	var step domain.NextStep
	if request.LoginPolicy.PasswordlessType != domain.PasswordlessTypeNotAllowed && user.IsPasswordlessReady() {
		if checkVerificationTimeMaxAge(userSession.PasswordlessVerification, repo.MultiFactorCheckLifeTime, request) {
			request.AuthTime = userSession.PasswordlessVerification
			return nil
		}
		step = &domain.PasswordlessStep{
			PasswordSet: user.PasswordSet,
		}
	}

	if user.PasswordlessInitRequired {
		return &domain.PasswordlessRegistrationPromptStep{}
	}

	if user.PasswordInitRequired {
		return &domain.InitPasswordStep{}
	}

	if checkVerificationTimeMaxAge(userSession.PasswordVerification, repo.PasswordCheckLifeTime, request) {
		request.PasswordVerified = true
		request.AuthTime = userSession.PasswordVerification
		return nil
	}
	if step != nil {
		return step
	}
	return &domain.PasswordStep{}
}

func (repo *AuthRequestRepo) mfaChecked(userSession *user_model.UserSessionView, request *domain.AuthRequest, user *user_model.UserView) (domain.NextStep, bool, error) {
	mfaLevel := request.MFALevel()
	allowedProviders, required := user.MFATypesAllowed(mfaLevel, request.LoginPolicy)
	promptRequired := (model.MFALevelToDomain(user.MFAMaxSetUp) < mfaLevel) || (len(allowedProviders) == 0 && required)
	if promptRequired || !repo.mfaSkippedOrSetUp(user) {
		types := user.MFATypesSetupPossible(mfaLevel, request.LoginPolicy)
		if promptRequired && len(types) == 0 {
			return nil, false, errors.ThrowPreconditionFailed(nil, "LOGIN-5Hm8s", "Errors.Login.LoginPolicy.MFA.ForceAndNotConfigured")
		}
		if len(types) == 0 {
			return nil, true, nil
		}
		return &domain.MFAPromptStep{
			Required:     promptRequired,
			MFAProviders: types,
		}, false, nil
	}
	switch mfaLevel {
	default:
		fallthrough
	case domain.MFALevelNotSetUp:
		if len(allowedProviders) == 0 {
			return nil, true, nil
		}
		fallthrough
	case domain.MFALevelSecondFactor:
		if checkVerificationTimeMaxAge(userSession.SecondFactorVerification, repo.SecondFactorCheckLifeTime, request) {
			request.MFAsVerified = append(request.MFAsVerified, model.MFATypeToDomain(userSession.SecondFactorVerificationType))
			request.AuthTime = userSession.SecondFactorVerification
			return nil, true, nil
		}
		fallthrough
	case domain.MFALevelMultiFactor:
		if checkVerificationTimeMaxAge(userSession.MultiFactorVerification, repo.MultiFactorCheckLifeTime, request) {
			request.MFAsVerified = append(request.MFAsVerified, model.MFATypeToDomain(userSession.MultiFactorVerificationType))
			request.AuthTime = userSession.MultiFactorVerification
			return nil, true, nil
		}
	}
	return &domain.MFAVerificationStep{
		MFAProviders: allowedProviders,
	}, false, nil
}

func (repo *AuthRequestRepo) mfaSkippedOrSetUp(user *user_model.UserView) bool {
	if user.MFAMaxSetUp > model.MFALevelNotSetUp {
		return true
	}
	return checkVerificationTime(user.MFAInitSkipped, repo.MFAInitSkippedLifeTime)
}

func (repo *AuthRequestRepo) getLoginPolicy(ctx context.Context, orgID string) (*iam_model.LoginPolicyView, error) {
	policy, err := repo.View.LoginPolicyByAggregateID(orgID)
	if err != nil {
		return nil, err
	}
	return iam_view_model.LoginPolicyViewToModel(policy), err
}

func (repo *AuthRequestRepo) getPrivacyPolicy(ctx context.Context, orgID string) (*domain.PrivacyPolicy, error) {
	policy, err := repo.View.PrivacyPolicyByAggregateID(orgID)
	if errors.IsNotFound(err) {
		policy, err = repo.View.PrivacyPolicyByAggregateID(repo.IAMID)
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			return policy.ToDomain(), nil
		}
		policy = &iam_view_model.PrivacyPolicyView{}
		events, err := repo.Eventstore.FilterEvents(ctx, es_models.NewSearchQuery().
			AggregateIDFilter(repo.IAMID).
			AggregateTypeFilter(iam.AggregateType).
			EventTypesFilter(es_models.EventType(iam.PrivacyPolicyAddedEventType), es_models.EventType(iam.PrivacyPolicyChangedEventType)))
		if err != nil || len(events) == 0 {
			return nil, errors.ThrowNotFound(err, "EVENT-GSRqg", "IAM.PrivacyPolicy.NotExisting")
		}
		policy.Default = true
		for _, event := range events {
			policy.AppendEvent(event)
		}
		return policy.ToDomain(), nil
	}
	if err != nil {
		return nil, err
	}
	return policy.ToDomain(), err
}

func (repo *AuthRequestRepo) getLockoutPolicy(ctx context.Context, orgID string) (*domain.LockoutPolicy, error) {
	policy, err := repo.View.LockoutPolicyByAggregateID(orgID)
	if errors.IsNotFound(err) {
		policy, err = repo.View.LockoutPolicyByAggregateID(repo.IAMID)
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if err == nil {
			return policy.ToDomain(), nil
		}
		policy = &iam_view_model.LockoutPolicyView{}
		events, err := repo.Eventstore.FilterEvents(ctx, es_models.NewSearchQuery().
			AggregateIDFilter(repo.IAMID).
			AggregateTypeFilter(iam.AggregateType).
			EventTypesFilter(es_models.EventType(iam.LockoutPolicyAddedEventType), es_models.EventType(iam.LockoutPolicyChangedEventType)))
		if err != nil || len(events) == 0 {
			return nil, errors.ThrowNotFound(err, "EVENT-Gfgr2", "IAM.LockoutPolicy.NotExisting")
		}
		policy.Default = true
		for _, event := range events {
			policy.AppendEvent(event)
		}
		return policy.ToDomain(), nil
	}
	if err != nil {
		return nil, err
	}
	return policy.ToDomain(), err
}

func (repo *AuthRequestRepo) getLabelPolicy(ctx context.Context, orgID string) (*domain.LabelPolicy, error) {
	policy, err := repo.View.LabelPolicyByAggregateIDAndState(orgID, int32(domain.LabelPolicyStateActive))
	if errors.IsNotFound(err) {
		policy, err = repo.View.LabelPolicyByAggregateIDAndState(repo.IAMID, int32(domain.LabelPolicyStateActive))
		if err != nil {
			return nil, err
		}
		policy.Default = true
	}
	if err != nil {
		return nil, err
	}
	return policy.ToDomain(), err
}

func (repo *AuthRequestRepo) getLoginTexts(ctx context.Context, aggregateID string) ([]*domain.CustomText, error) {
	loginTexts, err := repo.View.CustomTextsByAggregateIDAndTemplate(aggregateID, domain.LoginCustomText)
	if err != nil {
		return nil, err
	}
	return iam_view_model.CustomTextViewsToDomain(loginTexts), err
}

func setOrgID(orgViewProvider orgViewProvider, request *domain.AuthRequest) error {
	primaryDomain := request.GetScopeOrgPrimaryDomain()
	if primaryDomain == "" {
		return nil
	}

	org, err := orgViewProvider.OrgByPrimaryDomain(primaryDomain)
	if err != nil {
		return err
	}
	request.RequestedOrgID = org.ID
	request.RequestedOrgName = org.Name
	request.RequestedPrimaryDomain = primaryDomain
	return nil
}

func getLoginPolicyIDPProviders(provider idpProviderViewProvider, iamID, orgID string, defaultPolicy bool) ([]*iam_model.IDPProviderView, error) {
	if defaultPolicy {
		idpProviders, err := provider.IDPProvidersByAggregateIDAndState(iamID, iam_model.IDPConfigStateActive)
		if err != nil {
			return nil, err
		}
		return iam_view_model.IDPProviderViewsToModel(idpProviders), nil
	}
	idpProviders, err := provider.IDPProvidersByAggregateIDAndState(orgID, iam_model.IDPConfigStateActive)
	if err != nil {
		return nil, err
	}
	return iam_view_model.IDPProviderViewsToModel(idpProviders), nil
}

func checkVerificationTimeMaxAge(verificationTime time.Time, lifetime time.Duration, request *domain.AuthRequest) bool {
	if !checkVerificationTime(verificationTime, lifetime) {
		return false
	}
	if request.MaxAuthAge == nil {
		return true
	}
	return verificationTime.After(request.CreationDate.Add(-*request.MaxAuthAge))
}

func checkVerificationTime(verificationTime time.Time, lifetime time.Duration) bool {
	return verificationTime.Add(lifetime).After(time.Now().UTC())
}

func userSessionsByUserAgentID(provider userSessionViewProvider, agentID string) ([]*user_model.UserSessionView, error) {
	session, err := provider.UserSessionsByAgentID(agentID)
	if err != nil {
		return nil, err
	}
	return user_view_model.UserSessionsToModel(session, provider.PrefixAvatarURL()), nil
}

func userSessionByIDs(ctx context.Context, provider userSessionViewProvider, eventProvider userEventProvider, agentID string, user *user_model.UserView) (*user_model.UserSessionView, error) {
	session, err := provider.UserSessionByIDs(agentID, user.ID)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		session = &user_view_model.UserSessionView{UserAgentID: agentID, UserID: user.ID}
	}
	events, err := eventProvider.UserEventsByID(ctx, user.ID, session.Sequence)
	if err != nil {
		logging.Log("EVENT-Hse6s").WithError(err).WithField("traceID", tracing.TraceIDFromCtx(ctx)).Debug("error retrieving new events")
		return user_view_model.UserSessionToModel(session, provider.PrefixAvatarURL()), nil
	}
	sessionCopy := *session
	for _, event := range events {
		switch event.Type {
		case es_model.UserPasswordCheckSucceeded,
			es_model.UserPasswordCheckFailed,
			es_model.MFAOTPCheckSucceeded,
			es_model.MFAOTPCheckFailed,
			es_model.SignedOut,
			es_model.UserLocked,
			es_model.UserDeactivated,
			es_model.HumanPasswordCheckSucceeded,
			es_model.HumanPasswordCheckFailed,
			es_model.HumanExternalLoginCheckSucceeded,
			es_model.HumanMFAOTPCheckSucceeded,
			es_model.HumanMFAOTPCheckFailed,
			es_model.HumanSignedOut,
			es_model.HumanPasswordlessTokenCheckSucceeded,
			es_model.HumanPasswordlessTokenCheckFailed,
			es_model.HumanMFAU2FTokenCheckSucceeded,
			es_model.HumanMFAU2FTokenCheckFailed:
			eventData, err := user_view_model.UserSessionFromEvent(event)
			if err != nil {
				logging.Log("EVENT-sdgT3").WithError(err).WithField("traceID", tracing.TraceIDFromCtx(ctx)).Debug("error getting event data")
				return user_view_model.UserSessionToModel(session, provider.PrefixAvatarURL()), nil
			}
			if eventData.UserAgentID != agentID {
				continue
			}
		case es_model.UserRemoved:
			return nil, errors.ThrowPreconditionFailed(nil, "EVENT-dG2fe", "Errors.User.NotActive")
		}
		err := sessionCopy.AppendEvent(event)
		logging.Log("EVENT-qbhj3").OnError(err).WithField("traceID", tracing.TraceIDFromCtx(ctx)).Warn("error appending event")
	}
	return user_view_model.UserSessionToModel(&sessionCopy, provider.PrefixAvatarURL()), nil
}

func activeUserByID(ctx context.Context, userViewProvider userViewProvider, userEventProvider userEventProvider, orgViewProvider orgViewProvider, lockoutPolicyProvider lockoutPolicyViewProvider, userID string) (*user_model.UserView, error) {
	// PLANNED: Check LockoutPolicy
	user, err := userByID(ctx, userViewProvider, userEventProvider, userID)
	if err != nil {
		return nil, err
	}

	if user.HumanView == nil {
		return nil, errors.ThrowPreconditionFailed(nil, "EVENT-Lm69x", "Errors.User.NotHuman")
	}
	if user.State == user_model.UserStateLocked || user.State == user_model.UserStateSuspend {
		return nil, errors.ThrowPreconditionFailed(nil, "EVENT-FJ262", "Errors.User.Locked")
	}
	if !(user.State == user_model.UserStateActive || user.State == user_model.UserStateInitial) {
		return nil, errors.ThrowPreconditionFailed(nil, "EVENT-FJ262", "Errors.User.NotActive")
	}
	org, err := orgViewProvider.OrgByID(user.ResourceOwner)
	if err != nil {
		return nil, err
	}
	if org.State != int32(org_model.OrgStateActive) {
		return nil, errors.ThrowPreconditionFailed(nil, "EVENT-Zws3s", "Errors.User.NotActive")
	}
	return user, nil
}

func userByID(ctx context.Context, viewProvider userViewProvider, eventProvider userEventProvider, userID string) (*user_model.UserView, error) {
	user, viewErr := viewProvider.UserByID(userID)
	if viewErr != nil && !errors.IsNotFound(viewErr) {
		return nil, viewErr
	} else if user == nil {
		user = new(user_view_model.UserView)
	}
	events, err := eventProvider.UserEventsByID(ctx, userID, user.Sequence)
	if err != nil {
		logging.Log("EVENT-dfg42").WithError(err).WithField("traceID", tracing.TraceIDFromCtx(ctx)).Debug("error retrieving new events")
		return user_view_model.UserToModel(user, viewProvider.PrefixAvatarURL()), nil
	}
	if len(events) == 0 {
		if viewErr != nil {
			return nil, viewErr
		}
		return user_view_model.UserToModel(user, viewProvider.PrefixAvatarURL()), viewErr
	}
	userCopy := *user
	for _, event := range events {
		if err := userCopy.AppendEvent(event); err != nil {
			return user_view_model.UserToModel(user, viewProvider.PrefixAvatarURL()), nil
		}
	}
	if userCopy.State == int32(user_model.UserStateDeleted) {
		return nil, errors.ThrowNotFound(nil, "EVENT-3F9so", "Errors.User.NotFound")
	}
	return user_view_model.UserToModel(&userCopy, viewProvider.PrefixAvatarURL()), nil
}

func linkExternalIDPs(ctx context.Context, userCommandProvider userCommandProvider, request *domain.AuthRequest) error {
	externalIDPs := make([]*domain.ExternalIDP, len(request.LinkingUsers))
	for i, linkingUser := range request.LinkingUsers {
		externalIDP := &domain.ExternalIDP{
			ObjectRoot:     es_models.ObjectRoot{AggregateID: request.UserID},
			IDPConfigID:    linkingUser.IDPConfigID,
			ExternalUserID: linkingUser.ExternalUserID,
			DisplayName:    linkingUser.DisplayName,
		}
		externalIDPs[i] = externalIDP
	}
	data := authz.CtxData{
		UserID: "LOGIN",
		OrgID:  request.UserOrgID,
	}
	return userCommandProvider.BulkAddedHumanExternalIDP(authz.SetCtxData(ctx, data), request.UserID, request.UserOrgID, externalIDPs)
}

func linkingIDPConfigExistingInAllowedIDPs(linkingUsers []*domain.ExternalUser, idpProviders []*domain.IDPProvider) bool {
	for _, linkingUser := range linkingUsers {
		exists := false
		for _, idp := range idpProviders {
			if idp.IDPConfigID == linkingUser.IDPConfigID {
				exists = true
				continue
			}
		}
		if !exists {
			return false
		}
	}
	return true
}

func userGrantRequired(ctx context.Context, request *domain.AuthRequest, user *user_model.UserView, userGrantProvider userGrantProvider) (_ bool, err error) {
	var app *project_view_model.ApplicationView
	switch request.Request.Type() {
	case domain.AuthRequestTypeOIDC:
		app, err = userGrantProvider.ApplicationByClientID(ctx, request.ApplicationID)
		if err != nil {
			return false, err
		}
	default:
		return false, errors.ThrowPreconditionFailed(nil, "EVENT-dfrw2", "Errors.AuthRequest.RequestTypeNotSupported")
	}
	if !app.ProjectRoleCheck {
		return false, nil
	}
	grants, err := userGrantProvider.UserGrantsByProjectAndUserID(app.ProjectID, user.ID)
	if err != nil {
		return false, err
	}
	return len(grants) == 0, nil
}

func projectRequired(ctx context.Context, request *domain.AuthRequest, projectProvider projectProvider) (_ bool, err error) {
	var app *project_view_model.ApplicationView
	switch request.Request.Type() {
	case domain.AuthRequestTypeOIDC:
		app, err = projectProvider.ApplicationByClientID(ctx, request.ApplicationID)
		if err != nil {
			return false, err
		}
	default:
		return false, errors.ThrowPreconditionFailed(nil, "EVENT-dfrw2", "Errors.AuthRequest.RequestTypeNotSupported")
	}
	if !app.HasProjectCheck {
		return false, nil
	}
	_, err = projectProvider.OrgProjectMappingByIDs(request.UserOrgID, app.ProjectID)
	if errors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}
