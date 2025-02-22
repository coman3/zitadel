package model

import (
	"encoding/json"

	"github.com/caos/zitadel/internal/errors"
	es_models "github.com/caos/zitadel/internal/eventstore/v1/models"
	iam_model "github.com/caos/zitadel/internal/iam/model"
)

type PasswordComplexityPolicy struct {
	es_models.ObjectRoot

	State        int32  `json:"-"`
	MinLength    uint64 `json:"minLength"`
	HasLowercase bool   `json:"hasLowercase"`
	HasUppercase bool   `json:"hasUppercase"`
	HasNumber    bool   `json:"hasNumber"`
	HasSymbol    bool   `json:"hasSymbol"`
}

func PasswordComplexityPolicyToModel(policy *PasswordComplexityPolicy) *iam_model.PasswordComplexityPolicy {
	return &iam_model.PasswordComplexityPolicy{
		ObjectRoot:   policy.ObjectRoot,
		State:        iam_model.PolicyState(policy.State),
		MinLength:    policy.MinLength,
		HasLowercase: policy.HasLowercase,
		HasUppercase: policy.HasUppercase,
		HasNumber:    policy.HasNumber,
		HasSymbol:    policy.HasSymbol,
	}
}

func (i *IAM) appendAddPasswordComplexityPolicyEvent(event *es_models.Event) error {
	i.DefaultPasswordComplexityPolicy = new(PasswordComplexityPolicy)
	err := i.DefaultPasswordComplexityPolicy.SetData(event)
	if err != nil {
		return err
	}
	i.DefaultPasswordComplexityPolicy.ObjectRoot.CreationDate = event.CreationDate
	return nil
}

func (i *IAM) appendChangePasswordComplexityPolicyEvent(event *es_models.Event) error {
	return i.DefaultPasswordComplexityPolicy.SetData(event)
}

func (p *PasswordComplexityPolicy) SetData(event *es_models.Event) error {
	err := json.Unmarshal(event.Data, p)
	if err != nil {
		return errors.ThrowInternal(err, "EVENT-7JS9d", "unable to unmarshal data")
	}
	return nil
}
