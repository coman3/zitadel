package projection

import (
	"context"

	"github.com/caos/logging"
	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/errors"
	"github.com/caos/zitadel/internal/eventstore"
	"github.com/caos/zitadel/internal/eventstore/handler"
	"github.com/caos/zitadel/internal/eventstore/handler/crdb"
	"github.com/caos/zitadel/internal/repository/iam"
	"github.com/caos/zitadel/internal/repository/org"
	"github.com/caos/zitadel/internal/repository/policy"
)

type IDPLoginPolicyLinkProjection struct {
	crdb.StatementHandler
}

const (
	IDPLoginPolicyLinkTable = "zitadel.projections.idp_login_policy_links"
)

func NewIDPLoginPolicyLinkProjection(ctx context.Context, config crdb.StatementHandlerConfig) *IDPLoginPolicyLinkProjection {
	p := &IDPLoginPolicyLinkProjection{}
	config.ProjectionName = IDPLoginPolicyLinkTable
	config.Reducers = p.reducers()
	p.StatementHandler = crdb.NewStatementHandler(ctx, config)
	return p
}

func (p *IDPLoginPolicyLinkProjection) reducers() []handler.AggregateReducer {
	return []handler.AggregateReducer{
		{
			Aggregate: org.AggregateType,
			EventRedusers: []handler.EventReducer{
				{
					Event:  org.LoginPolicyIDPProviderAddedEventType,
					Reduce: p.reduceAdded,
				},
				{
					Event:  org.LoginPolicyIDPProviderCascadeRemovedEventType,
					Reduce: p.reduceCascadeRemoved,
				},
				{
					Event:  org.LoginPolicyIDPProviderRemovedEventType,
					Reduce: p.reduceRemoved,
				},
			},
		},
		{
			Aggregate: iam.AggregateType,
			EventRedusers: []handler.EventReducer{
				{
					Event:  iam.LoginPolicyIDPProviderAddedEventType,
					Reduce: p.reduceAdded,
				},
				{
					Event:  iam.LoginPolicyIDPProviderCascadeRemovedEventType,
					Reduce: p.reduceCascadeRemoved,
				},
				{
					Event:  iam.LoginPolicyIDPProviderRemovedEventType,
					Reduce: p.reduceRemoved,
				},
			},
		},
	}
}

const (
	IDPLoginPolicyLinkIDPIDCol         = "idp_id"
	IDPLoginPolicyLinkAggregateIDCol   = "aggregate_id"
	IDPLoginPolicyLinkCreationDateCol  = "creation_date"
	IDPLoginPolicyLinkChangeDateCol    = "change_date"
	IDPLoginPolicyLinkSequenceCol      = "sequence"
	IDPLoginPolicyLinkResourceOwnerCol = "resource_owner"
	IDPLoginPolicyLinkProviderTypeCol  = "provider_type"
)

func (p *IDPLoginPolicyLinkProjection) reduceAdded(event eventstore.EventReader) (*handler.Statement, error) {
	var (
		idp          policy.IdentityProviderAddedEvent
		providerType domain.IdentityProviderType
	)

	switch e := event.(type) {
	case *org.IdentityProviderAddedEvent:
		idp = e.IdentityProviderAddedEvent
		providerType = domain.IdentityProviderTypeOrg
	case *iam.IdentityProviderAddedEvent:
		idp = e.IdentityProviderAddedEvent
		providerType = domain.IdentityProviderTypeSystem
	default:
		logging.LogWithFields("HANDL-oce92", "seq", event.Sequence(), "expectedTypes", []eventstore.EventType{org.LoginPolicyIDPProviderAddedEventType, iam.LoginPolicyIDPProviderAddedEventType}).Error("wrong event type")
		return nil, errors.ThrowInvalidArgument(nil, "HANDL-Nlp55", "reduce.wrong.event.type")
	}
	return crdb.NewCreateStatement(&idp,
		[]handler.Column{
			handler.NewCol(IDPLoginPolicyLinkIDPIDCol, idp.IDPConfigID),
			handler.NewCol(IDPLoginPolicyLinkAggregateIDCol, idp.Aggregate().ID),
			handler.NewCol(IDPLoginPolicyLinkCreationDateCol, idp.CreationDate()),
			handler.NewCol(IDPLoginPolicyLinkChangeDateCol, idp.CreationDate()),
			handler.NewCol(IDPLoginPolicyLinkSequenceCol, idp.Sequence()),
			handler.NewCol(IDPLoginPolicyLinkResourceOwnerCol, idp.Aggregate().ResourceOwner),
			handler.NewCol(IDPLoginPolicyLinkProviderTypeCol, providerType),
		},
	), nil
}

func (p *IDPLoginPolicyLinkProjection) reduceRemoved(event eventstore.EventReader) (*handler.Statement, error) {
	var idp policy.IdentityProviderRemovedEvent
	switch e := event.(type) {
	case *org.IdentityProviderRemovedEvent:
		idp = e.IdentityProviderRemovedEvent
	case *iam.IdentityProviderRemovedEvent:
		idp = e.IdentityProviderRemovedEvent
	default:
		logging.LogWithFields("HANDL-vAH3I", "seq", event.Sequence(), "expectedTypes", []eventstore.EventType{org.LoginPolicyIDPProviderRemovedEventType, iam.LoginPolicyIDPProviderRemovedEventType}).Error("wrong event type")
		return nil, errors.ThrowInvalidArgument(nil, "HANDL-tUMYY", "reduce.wrong.event.type")
	}
	return crdb.NewDeleteStatement(&idp,
		[]handler.Condition{
			handler.NewCond(IDPLoginPolicyLinkIDPIDCol, idp.IDPConfigID),
			handler.NewCond(IDPLoginPolicyLinkAggregateIDCol, idp.Aggregate().ID),
		},
	), nil
}

func (p *IDPLoginPolicyLinkProjection) reduceCascadeRemoved(event eventstore.EventReader) (*handler.Statement, error) {
	var idp policy.IdentityProviderCascadeRemovedEvent
	switch e := event.(type) {
	case *org.IdentityProviderCascadeRemovedEvent:
		idp = e.IdentityProviderCascadeRemovedEvent
	case *iam.IdentityProviderCascadeRemovedEvent:
		idp = e.IdentityProviderCascadeRemovedEvent
	default:
		logging.LogWithFields("HANDL-7lZaf", "seq", event.Sequence(), "expectedTypes", []eventstore.EventType{org.LoginPolicyIDPProviderCascadeRemovedEventType, iam.LoginPolicyIDPProviderCascadeRemovedEventType}).Error("wrong event type")
		return nil, errors.ThrowInvalidArgument(nil, "HANDL-iCKSj", "reduce.wrong.event.type")
	}
	return crdb.NewDeleteStatement(&idp,
		[]handler.Condition{
			handler.NewCond(IDPLoginPolicyLinkIDPIDCol, idp.IDPConfigID),
			handler.NewCond(IDPLoginPolicyLinkAggregateIDCol, idp.Aggregate().ID),
		},
	), nil
}
