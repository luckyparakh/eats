package http

import (
	"context"

	"eats/backend/common"
	"eats/backend/common/shared"
	"eats/backend/orders/app"
)

type Handler struct {
	svc *app.Service
}

func NewHandler(svc *app.Service) Handler {
	return Handler{
		svc: svc,
	}
}

func (h Handler) RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) {
	customer := request.Body
	customerUUID := common.NewUUIDv7()

	address, err := addressfromOpenAPIToShared(customer.Address)
	if err != nil {
		return nil, common.NewInvalidInputError("invalid-address", "invalid address: %s", err)
	}

	customerApp := app.Customer{
		CustomerUUID: app.CustomerUUID{UUID: customerUUID},
		Name:         customer.Name,
		Email:        string(customer.Email),
		PhoneNumber:  customer.PhoneNumber,
		Address:      address,
	}

	if err := h.svc.RegisterCustomer(ctx, customerApp); err != nil {
		return nil, err
	}

	return RegisterCustomer201JSONResponse{
		CustomerUuid: app.CustomerUUID{UUID: customerUUID},
	}, nil
}

func Register(ctx context.Context, e EchoRouter, handler Handler) error {
	RegisterHandlers(e, NewStrictHandler(handler, nil))

	return nil
}

func addressfromOpenAPIToShared(addr Address) (shared.Address, error) {
	return shared.NewAddress(
		addr.Line1,
		addr.Line2,
		addr.PostalCode,
		addr.City,
		addr.CountryCode,
	)
}
