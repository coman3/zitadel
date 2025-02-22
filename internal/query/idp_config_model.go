package query

import (
	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/eventstore"
	"github.com/caos/zitadel/internal/repository/idpconfig"
)

type IDPConfigReadModel struct {
	eventstore.ReadModel

	State        domain.IDPConfigState
	ConfigID     string
	Name         string
	AutoRegister bool
	StylingType  domain.IDPConfigStylingType
	ProviderType domain.IdentityProviderType

	OIDCConfig *OIDCConfigReadModel
	JWTConfig  *JWTConfigReadModel
}

func NewIDPConfigReadModel(configID string) *IDPConfigReadModel {
	return &IDPConfigReadModel{
		ConfigID: configID,
	}
}

func (rm *IDPConfigReadModel) AppendEvents(events ...eventstore.EventReader) {
	for _, event := range events {
		switch e := event.(type) {
		case *idpconfig.IDPConfigAddedEvent:
			rm.ReadModel.AppendEvents(e)
		case *idpconfig.IDPConfigChangedEvent:
			rm.ReadModel.AppendEvents(e)
		case *idpconfig.IDPConfigDeactivatedEvent:
			rm.ReadModel.AppendEvents(e)
		case *idpconfig.IDPConfigReactivatedEvent:
			rm.ReadModel.AppendEvents(e)
		case *idpconfig.IDPConfigRemovedEvent:
			rm.ReadModel.AppendEvents(e)
		case *idpconfig.OIDCConfigAddedEvent:
			rm.OIDCConfig = &OIDCConfigReadModel{}
			rm.ReadModel.AppendEvents(e)
			rm.OIDCConfig.AppendEvents(event)
		case *idpconfig.OIDCConfigChangedEvent:
			rm.ReadModel.AppendEvents(e)
			rm.OIDCConfig.AppendEvents(event)
		case *idpconfig.JWTConfigAddedEvent:
			rm.JWTConfig = &JWTConfigReadModel{}
			rm.ReadModel.AppendEvents(e)
			rm.JWTConfig.AppendEvents(event)
		case *idpconfig.JWTConfigChangedEvent:
			rm.ReadModel.AppendEvents(e)
			rm.JWTConfig.AppendEvents(event)
		}
	}
}

func (rm *IDPConfigReadModel) Reduce() error {
	for _, event := range rm.Events {
		switch e := event.(type) {
		case *idpconfig.IDPConfigAddedEvent:
			rm.reduceConfigAddedEvent(e)
		case *idpconfig.IDPConfigChangedEvent:
			rm.reduceConfigChangedEvent(e)
		case *idpconfig.IDPConfigDeactivatedEvent:
			rm.reduceConfigStateChanged(e.ConfigID, domain.IDPConfigStateInactive)
		case *idpconfig.IDPConfigReactivatedEvent:
			rm.reduceConfigStateChanged(e.ConfigID, domain.IDPConfigStateActive)
		case *idpconfig.IDPConfigRemovedEvent:
			rm.reduceConfigStateChanged(e.ConfigID, domain.IDPConfigStateRemoved)
		}
	}

	if rm.OIDCConfig != nil {
		if err := rm.OIDCConfig.Reduce(); err != nil {
			return err
		}
	}
	if rm.JWTConfig != nil {
		if err := rm.JWTConfig.Reduce(); err != nil {
			return err
		}
	}
	return rm.ReadModel.Reduce()
}

func (rm *IDPConfigReadModel) reduceConfigAddedEvent(e *idpconfig.IDPConfigAddedEvent) {
	rm.ConfigID = e.ConfigID
	rm.Name = e.Name
	rm.StylingType = e.StylingType
	rm.State = domain.IDPConfigStateActive
	rm.AutoRegister = e.AutoRegister
}

func (rm *IDPConfigReadModel) reduceConfigChangedEvent(e *idpconfig.IDPConfigChangedEvent) {
	if e.Name != nil {
		rm.Name = *e.Name
	}
	if e.StylingType != nil && e.StylingType.Valid() {
		rm.StylingType = *e.StylingType
	}
	if e.AutoRegister != nil {
		rm.AutoRegister = *e.AutoRegister
	}
}

func (rm *IDPConfigReadModel) reduceConfigStateChanged(configID string, state domain.IDPConfigState) {
	rm.State = state
}
