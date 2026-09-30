package http

import (
	"context"

	"eats/backend/common"
)

type Handler struct{}

func NewHandler() Handler {
	return Handler{}
}

func (h Handler) RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) {
	customerUUID := common.NewUUIDv7()

	return RegisterCustomer201JSONResponse{
		CustomerUuid: customerUUID,
	}, nil
}

func Register(ctx context.Context, e EchoRouter, handler Handler) error {
	RegisterHandlers(e, NewStrictHandler(handler, nil))

	return nil
}
