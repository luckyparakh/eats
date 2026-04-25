package http

import (
	"context"

	"eats/backend/common"

	"github.com/google/uuid"
)

type Handler struct{}

func NewHandler() Handler {
	return Handler{}
}

func Register(ctx context.Context, e common.EchoRouter, handler Handler) error {
	RegisterHandlers(e, NewStrictHandler(&handler, nil))
	return nil
}

func (h *Handler) RegisterCustomer(
	ctx context.Context,
	request RegisterCustomerRequestObject,
) (RegisterCustomerResponseObject, error) {
	return RegisterCustomer201JSONResponse{
		CustomerUuid: uuid.New(),
	}, nil
}
