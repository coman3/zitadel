package crdb

import (
	"github.com/caos/zitadel/internal/eventstore"
	"github.com/caos/zitadel/internal/eventstore/handler"
)

//reduce implements handler.Reduce function
func (h *StatementHandler) reduce(event eventstore.EventReader) (*handler.Statement, error) {
	reduce, ok := h.reduces[event.Type()]
	if !ok {
		return NewNoOpStatement(event), nil
	}

	return reduce(event)
}
