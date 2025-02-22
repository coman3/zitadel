package projection

import (
	"context"

	"github.com/caos/logging"
	"github.com/caos/zitadel/internal/errors"
	"github.com/caos/zitadel/internal/eventstore"
	"github.com/caos/zitadel/internal/eventstore/handler"
	"github.com/caos/zitadel/internal/eventstore/handler/crdb"
	"github.com/caos/zitadel/internal/repository/user"
)

type IDPUserLinkProjection struct {
	crdb.StatementHandler
}

func NewIDPUserLinkProjection(ctx context.Context, config crdb.StatementHandlerConfig) *IDPUserLinkProjection {
	p := &IDPUserLinkProjection{}
	config.ProjectionName = IDPUserLinkTable
	config.Reducers = p.reducers()
	p.StatementHandler = crdb.NewStatementHandler(ctx, config)
	return p
}

func (p *IDPUserLinkProjection) reducers() []handler.AggregateReducer {
	return []handler.AggregateReducer{
		{
			Aggregate: user.AggregateType,
			EventRedusers: []handler.EventReducer{
				{
					Event:  user.UserIDPLinkAddedType,
					Reduce: p.reduceAdded,
				},
				{
					Event:  user.UserIDPLinkCascadeRemovedType,
					Reduce: p.reduceCascadeRemoved,
				},
				{
					Event:  user.UserIDPLinkRemovedType,
					Reduce: p.reduceRemoved,
				},
			},
		},
	}
}

const (
	IDPUserLinkTable             = "zitadel.projections.idp_user_links"
	IDPUserLinkIDPIDCol          = "idp_id"
	IDPUserLinkUserIDCol         = "user_id"
	IDPUserLinkExternalUserIDCol = "external_user_id"
	IDPUserLinkCreationDateCol   = "creation_date"
	IDPUserLinkChangeDateCol     = "change_date"
	IDPUserLinkSequenceCol       = "sequence"
	IDPUserLinkResourceOwnerCol  = "resource_owner"
	IDPUserLinkDisplayNameCol    = "display_name"
)

func (p *IDPUserLinkProjection) reduceAdded(event eventstore.EventReader) (*handler.Statement, error) {
	e, ok := event.(*user.UserIDPLinkAddedEvent)
	if !ok {
		logging.LogWithFields("HANDL-v2qC3", "seq", event.Sequence(), "expectedType", user.UserIDPLinkAddedType).Error("wrong event type")
		return nil, errors.ThrowInvalidArgument(nil, "HANDL-DpmXq", "reduce.wrong.event.type")
	}
	return crdb.NewCreateStatement(e,
		[]handler.Column{
			handler.NewCol(IDPUserLinkIDPIDCol, e.IDPConfigID),
			handler.NewCol(IDPUserLinkUserIDCol, e.Aggregate().ID),
			handler.NewCol(IDPUserLinkExternalUserIDCol, e.ExternalUserID),
			handler.NewCol(IDPUserLinkCreationDateCol, e.CreationDate()),
			handler.NewCol(IDPUserLinkChangeDateCol, e.CreationDate()),
			handler.NewCol(IDPUserLinkSequenceCol, e.Sequence()),
			handler.NewCol(IDPUserLinkResourceOwnerCol, e.Aggregate().ResourceOwner),
			handler.NewCol(IDPUserLinkDisplayNameCol, e.DisplayName),
		},
	), nil
}

func (p *IDPUserLinkProjection) reduceRemoved(event eventstore.EventReader) (*handler.Statement, error) {
	e, ok := event.(*user.UserIDPLinkRemovedEvent)
	if !ok {
		logging.LogWithFields("HANDL-zX5m9", "seq", event.Sequence(), "expectedType", user.UserIDPLinkRemovedType).Error("wrong event type")
		return nil, errors.ThrowInvalidArgument(nil, "HANDL-AZmfJ", "reduce.wrong.event.type")
	}
	return crdb.NewDeleteStatement(e,
		[]handler.Condition{
			handler.NewCond(IDPUserLinkIDPIDCol, e.IDPConfigID),
			handler.NewCond(IDPUserLinkUserIDCol, e.Aggregate().ID),
			handler.NewCond(IDPUserLinkExternalUserIDCol, e.ExternalUserID),
		},
	), nil
}

func (p *IDPUserLinkProjection) reduceCascadeRemoved(event eventstore.EventReader) (*handler.Statement, error) {
	e, ok := event.(*user.UserIDPLinkCascadeRemovedEvent)
	if !ok {
		logging.LogWithFields("HANDL-I0s2H", "seq", event.Sequence(), "expectedType", user.UserIDPLinkCascadeRemovedType).Error("wrong event type")
		return nil, errors.ThrowInvalidArgument(nil, "HANDL-jQpv9", "reduce.wrong.event.type")
	}
	return crdb.NewDeleteStatement(e,
		[]handler.Condition{
			handler.NewCond(IDPUserLinkIDPIDCol, e.IDPConfigID),
			handler.NewCond(IDPUserLinkUserIDCol, e.Aggregate().ID),
			handler.NewCond(IDPUserLinkExternalUserIDCol, e.ExternalUserID),
		},
	), nil
}
