package object

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/caos/zitadel/internal/domain"
	"github.com/caos/zitadel/internal/query"
	object_pb "github.com/caos/zitadel/pkg/grpc/object"
)

func DomainToChangeDetailsPb(objectDetail *domain.ObjectDetails) *object_pb.ObjectDetails {
	return &object_pb.ObjectDetails{
		Sequence:      objectDetail.Sequence,
		ChangeDate:    timestamppb.New(objectDetail.EventDate),
		ResourceOwner: objectDetail.ResourceOwner,
	}
}

func DomainToAddDetailsPb(objectDetail *domain.ObjectDetails) *object_pb.ObjectDetails {
	return &object_pb.ObjectDetails{
		Sequence:      objectDetail.Sequence,
		CreationDate:  timestamppb.New(objectDetail.EventDate),
		ResourceOwner: objectDetail.ResourceOwner,
	}
}

func ToViewDetailsPb(
	sequence uint64,
	creationDate,
	changeDate time.Time,
	resourceOwner string,
) *object_pb.ObjectDetails {
	return &object_pb.ObjectDetails{
		Sequence:      sequence,
		CreationDate:  timestamppb.New(creationDate),
		ChangeDate:    timestamppb.New(changeDate),
		ResourceOwner: resourceOwner,
	}
}

func ChangeToDetailsPb(
	sequence uint64,
	changeDate time.Time,
	resourceOwner string,
) *object_pb.ObjectDetails {
	return &object_pb.ObjectDetails{
		Sequence:      sequence,
		ChangeDate:    timestamppb.New(changeDate),
		ResourceOwner: resourceOwner,
	}
}

func AddToDetailsPb(
	sequence uint64,
	creationDate time.Time,
	resourceOwner string,
) *object_pb.ObjectDetails {
	return &object_pb.ObjectDetails{
		Sequence:      sequence,
		CreationDate:  timestamppb.New(creationDate),
		ResourceOwner: resourceOwner,
	}
}

func ToListDetails(
	totalResult,
	processedSequence uint64,
	viewTimestamp time.Time,
) *object_pb.ListDetails {
	return &object_pb.ListDetails{
		TotalResult:       totalResult,
		ProcessedSequence: processedSequence,
		ViewTimestamp:     timestamppb.New(viewTimestamp),
	}
}

func TextMethodToModel(method object_pb.TextQueryMethod) domain.SearchMethod {
	switch method {
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_EQUALS:
		return domain.SearchMethodEquals
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE:
		return domain.SearchMethodEqualsIgnoreCase
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_STARTS_WITH:
		return domain.SearchMethodStartsWith
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_STARTS_WITH_IGNORE_CASE:
		return domain.SearchMethodStartsWithIgnoreCase
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_CONTAINS:
		return domain.SearchMethodContains
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_CONTAINS_IGNORE_CASE:
		return domain.SearchMethodContainsIgnoreCase
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_ENDS_WITH:
		return domain.SearchMethodEndsWith
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_ENDS_WITH_IGNORE_CASE:
		return domain.SearchMethodEndsWithIgnoreCase
	default:
		return -1
	}
}

func TextMethodToQuery(method object_pb.TextQueryMethod) query.TextComparison {
	switch method {
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_EQUALS:
		return query.TextEquals
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_EQUALS_IGNORE_CASE:
		return query.TextEqualsIgnoreCase
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_STARTS_WITH:
		return query.TextStartsWith
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_STARTS_WITH_IGNORE_CASE:
		return query.TextStartsWithIgnoreCase
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_CONTAINS:
		return query.TextContains
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_CONTAINS_IGNORE_CASE:
		return query.TextContainsIgnoreCase
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_ENDS_WITH:
		return query.TextEndsWith
	case object_pb.TextQueryMethod_TEXT_QUERY_METHOD_ENDS_WITH_IGNORE_CASE:
		return query.TextEndsWithIgnoreCase
	default:
		return -1
	}
}

func ListQueryToModel(query *object_pb.ListQuery) (offset, limit uint64, asc bool) {
	if query == nil {
		return
	}
	return query.Offset, uint64(query.Limit), query.Asc
}
