package command

import (
	"context"
	"reflect"

	"github.com/caos/zitadel/internal/eventstore"

	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/repository/iam"
	"github.com/caos/zitadel/internal/repository/policy"
)

type IAMMailTemplateWriteModel struct {
	MailTemplateWriteModel
}

func NewIAMMailTemplateWriteModel() *IAMMailTemplateWriteModel {
	return &IAMMailTemplateWriteModel{
		MailTemplateWriteModel{
			WriteModel: eventstore.WriteModel{
				AggregateID:   domain.IAMID,
				ResourceOwner: domain.IAMID,
			},
		},
	}
}

func (wm *IAMMailTemplateWriteModel) AppendEvents(events ...eventstore.EventReader) {
	for _, event := range events {
		switch e := event.(type) {
		case *iam.MailTemplateAddedEvent:
			wm.MailTemplateWriteModel.AppendEvents(&e.MailTemplateAddedEvent)
		case *iam.MailTemplateChangedEvent:
			wm.MailTemplateWriteModel.AppendEvents(&e.MailTemplateChangedEvent)
		}
	}
}

func (wm *IAMMailTemplateWriteModel) Reduce() error {
	return wm.MailTemplateWriteModel.Reduce()
}

func (wm *IAMMailTemplateWriteModel) Query() *eventstore.SearchQueryBuilder {
	return eventstore.NewSearchQueryBuilder(eventstore.ColumnsEvent).
		ResourceOwner(wm.ResourceOwner).
		AddQuery().
		AggregateTypes(iam.AggregateType).
		AggregateIDs(wm.MailTemplateWriteModel.AggregateID).
		EventTypes(
			iam.MailTemplateAddedEventType,
			iam.MailTemplateChangedEventType).
		Builder()
}

func (wm *IAMMailTemplateWriteModel) NewChangedEvent(
	ctx context.Context,
	aggregate *eventstore.Aggregate,
	template []byte,
) (*iam.MailTemplateChangedEvent, bool) {
	changes := make([]policy.MailTemplateChanges, 0)
	if !reflect.DeepEqual(wm.Template, template) {
		changes = append(changes, policy.ChangeTemplate(template))
	}
	if len(changes) == 0 {
		return nil, false
	}
	changedEvent, err := iam.NewMailTemplateChangedEvent(ctx, aggregate, changes)
	if err != nil {
		return nil, false
	}
	return changedEvent, true
}
