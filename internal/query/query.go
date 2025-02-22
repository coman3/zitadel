package query

import (
	"context"
	"database/sql"

	sd "github.com/caos/zitadel/internal/config/systemdefaults"
	"github.com/caos/zitadel/internal/config/types"
	"github.com/caos/zitadel/internal/eventstore"
	iam_model "github.com/caos/zitadel/internal/iam/model"
	"github.com/caos/zitadel/internal/query/projection"
	"github.com/caos/zitadel/internal/repository/action"
	iam_repo "github.com/caos/zitadel/internal/repository/iam"
	"github.com/caos/zitadel/internal/repository/keypair"
	"github.com/caos/zitadel/internal/repository/org"
	"github.com/caos/zitadel/internal/repository/project"
	usr_repo "github.com/caos/zitadel/internal/repository/user"
	"github.com/caos/zitadel/internal/telemetry/tracing"
)

type Queries struct {
	iamID      string
	eventstore *eventstore.Eventstore
	client     *sql.DB
}

type Config struct {
	Eventstore types.SQLUser
}

func StartQueries(ctx context.Context, es *eventstore.Eventstore, projections projection.Config, defaults sd.SystemDefaults) (repo *Queries, err error) {
	sqlClient, err := projections.CRDB.Start()
	if err != nil {
		return nil, err
	}

	repo = &Queries{
		iamID:      defaults.IamID,
		eventstore: es,
		client:     sqlClient,
	}
	iam_repo.RegisterEventMappers(repo.eventstore)
	usr_repo.RegisterEventMappers(repo.eventstore)
	org.RegisterEventMappers(repo.eventstore)
	project.RegisterEventMappers(repo.eventstore)
	action.RegisterEventMappers(repo.eventstore)
	keypair.RegisterEventMappers(repo.eventstore)

	err = projection.Start(ctx, sqlClient, es, projections, defaults)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *Queries) IAMByID(ctx context.Context, id string) (_ *iam_model.IAM, err error) {
	readModel, err := r.iamByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return readModelToIAM(readModel), nil
}

func (r *Queries) iamByID(ctx context.Context, id string) (_ *ReadModel, err error) {
	ctx, span := tracing.NewSpan(ctx)
	defer func() { span.EndWithError(err) }()

	readModel := NewReadModel(id)
	err = r.eventstore.FilterToQueryReducer(ctx, readModel)
	if err != nil {
		return nil, err
	}

	return readModel, nil
}
