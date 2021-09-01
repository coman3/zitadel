package command

import (
	"context"

	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/eventstore"
	"github.com/caos/zitadel/internal/repository/project"
)

type ProjectWriteModel struct {
	eventstore.WriteModel

	Name                           string
	ProjectRoleAssertion           bool
	ProjectRoleCheck               bool
	HasProjectCheck                bool
	RegisterOnProjectResourceOwner bool
	PrivateLabelingSetting         domain.PrivateLabelingSetting
	LoginPolicySetting             domain.LoginPolicySetting
	State                          domain.ProjectState
}

func NewProjectWriteModel(projectID string, resourceOwner string) *ProjectWriteModel {
	return &ProjectWriteModel{
		WriteModel: eventstore.WriteModel{
			AggregateID:   projectID,
			ResourceOwner: resourceOwner,
		},
	}
}

func (wm *ProjectWriteModel) Reduce() error {
	for _, event := range wm.Events {
		switch e := event.(type) {
		case *project.ProjectAddedEvent:
			wm.Name = e.Name
			wm.ProjectRoleAssertion = e.ProjectRoleAssertion
			wm.ProjectRoleCheck = e.ProjectRoleCheck
			wm.HasProjectCheck = e.HasProjectCheck
			wm.RegisterOnProjectResourceOwner = e.RegisterOnProjectResourceOwner
			wm.PrivateLabelingSetting = e.PrivateLabelingSetting
			wm.LoginPolicySetting = e.LoginPolicySetting
			wm.State = domain.ProjectStateActive
		case *project.ProjectChangeEvent:
			if e.Name != nil {
				wm.Name = *e.Name
			}
			if e.ProjectRoleAssertion != nil {
				wm.ProjectRoleAssertion = *e.ProjectRoleAssertion
			}
			if e.ProjectRoleCheck != nil {
				wm.ProjectRoleCheck = *e.ProjectRoleCheck
			}
			if e.HasProjectCheck != nil {
				wm.HasProjectCheck = *e.HasProjectCheck
			}
			if e.RegisterOnProjectResourceOwner != nil {
				wm.RegisterOnProjectResourceOwner = *e.RegisterOnProjectResourceOwner
			}
			if e.PrivateLabelingSetting != nil {
				wm.PrivateLabelingSetting = *e.PrivateLabelingSetting
			}
			if e.LoginPolicySetting != nil {
				wm.LoginPolicySetting = *e.LoginPolicySetting
			}
		case *project.ProjectDeactivatedEvent:
			if wm.State == domain.ProjectStateRemoved {
				continue
			}
			wm.State = domain.ProjectStateInactive
		case *project.ProjectReactivatedEvent:
			if wm.State == domain.ProjectStateRemoved {
				continue
			}
			wm.State = domain.ProjectStateActive
		case *project.ProjectRemovedEvent:
			wm.State = domain.ProjectStateRemoved
		}
	}
	return wm.WriteModel.Reduce()
}

func (wm *ProjectWriteModel) Query() *eventstore.SearchQueryBuilder {
	return eventstore.NewSearchQueryBuilder(eventstore.ColumnsEvent).
		ResourceOwner(wm.ResourceOwner).
		AddQuery().
		AggregateTypes(project.AggregateType).
		AggregateIDs(wm.AggregateID).
		EventTypes(project.ProjectAddedType,
			project.ProjectChangedType,
			project.ProjectDeactivatedType,
			project.ProjectReactivatedType,
			project.ProjectRemovedType).
		Builder()
}

func (wm *ProjectWriteModel) NewChangedEvent(
	ctx context.Context,
	aggregate *eventstore.Aggregate,
	name string,
	projectRoleAssertion,
	projectRoleCheck,
	hasProjectCheck,
	registerOnProjectResourceOwner bool,
	privateLabelingSetting domain.PrivateLabelingSetting,
	loginPolicySetting domain.LoginPolicySetting,
) (*project.ProjectChangeEvent, bool, error) {
	changes := make([]project.ProjectChanges, 0)
	var err error

	oldName := ""
	if wm.Name != name {
		oldName = wm.Name
		changes = append(changes, project.ChangeName(name))
	}
	if wm.ProjectRoleAssertion != projectRoleAssertion {
		changes = append(changes, project.ChangeProjectRoleAssertion(projectRoleAssertion))
	}
	if wm.ProjectRoleCheck != projectRoleCheck {
		changes = append(changes, project.ChangeProjectRoleCheck(projectRoleCheck))
	}
	if wm.HasProjectCheck != hasProjectCheck {
		changes = append(changes, project.ChangeHasProjectCheck(hasProjectCheck))
	}
	if wm.RegisterOnProjectResourceOwner != registerOnProjectResourceOwner {
		changes = append(changes, project.ChangeRegisterOnProjectResourceOwner(registerOnProjectResourceOwner))
	}
	if wm.PrivateLabelingSetting != privateLabelingSetting {
		changes = append(changes, project.ChangePrivateLabelingSetting(privateLabelingSetting))
	}
	if wm.LoginPolicySetting != loginPolicySetting {
		changes = append(changes, project.ChangeLoginPolicySetting(loginPolicySetting))
	}
	if len(changes) == 0 {
		return nil, false, nil
	}
	changeEvent, err := project.NewProjectChangeEvent(ctx, aggregate, oldName, changes)
	if err != nil {
		return nil, false, err
	}
	return changeEvent, true, nil
}

func ProjectAggregateFromWriteModel(wm *eventstore.WriteModel) *eventstore.Aggregate {
	return eventstore.AggregateFromWriteModel(wm, project.AggregateType, project.AggregateVersion)
}
