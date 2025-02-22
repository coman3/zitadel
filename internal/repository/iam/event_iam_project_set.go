package iam

import (
	"context"
	"encoding/json"
	"github.com/caos/zitadel/internal/eventstore"

	"github.com/caos/zitadel/internal/errors"
	"github.com/caos/zitadel/internal/eventstore/repository"
)

const (
	ProjectSetEventType eventstore.EventType = "iam.project.iam.set"
)

type ProjectSetEvent struct {
	eventstore.BaseEvent `json:"-"`

	ProjectID string `json:"iamProjectId"`
}

func (e *ProjectSetEvent) Data() interface{} {
	return e
}

func (e *ProjectSetEvent) UniqueConstraints() []*eventstore.EventUniqueConstraint {
	return nil
}

func NewIAMProjectSetEvent(
	ctx context.Context,
	aggregate *eventstore.Aggregate,
	projectID string,
) *ProjectSetEvent {
	return &ProjectSetEvent{
		BaseEvent: *eventstore.NewBaseEventForPush(
			ctx,
			aggregate,
			ProjectSetEventType,
		),
		ProjectID: projectID,
	}
}

func ProjectSetMapper(event *repository.Event) (eventstore.EventReader, error) {
	e := &ProjectSetEvent{
		BaseEvent: *eventstore.BaseEventFromRepo(event),
	}
	err := json.Unmarshal(event.Data, e)
	if err != nil {
		return nil, errors.ThrowInternal(err, "IAM-cdFZH", "unable to unmarshal global org set")
	}

	return e, nil
}
