package http

import (
	"context"

	"eats/backend/common"
)

type CustomerRepository interface {
	RegisterCustomer(ctx context.Context, customerUUID common.UUID, customer RegisterCustomer) error
}

type Handler struct {
	customerRepository CustomerRepository
}

func NewHandler(cr CustomerRepository) Handler {
	if cr == nil {
		panic("CustomerRepository can't be nil")
	}
	return Handler{
		customerRepository: cr,
	}
}

func (h Handler) RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) {
	customer := request.Body
	customerUUID := common.NewUUIDv7()

	if err := h.customerRepository.RegisterCustomer(ctx, customerUUID, *customer); err != nil {
		return nil, err
	}

	return RegisterCustomer201JSONResponse{
		CustomerUuid: customerUUID,
	}, nil
}

func Register(ctx context.Context, e EchoRouter, handler Handler) error {
	RegisterHandlers(e, NewStrictHandler(handler, nil))

	return nil
}
