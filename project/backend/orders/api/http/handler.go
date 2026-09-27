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

// oapi-codegen generates two interfaces for each spec:
//
//   - ServerInterface: what Echo's router actually wires up.
//     RegisterCustomer(ctx echo.Context) error
//     Raw echo.Context in, error out — no typed parsing/encoding.
//
//   - StrictServerInterface: what we implement.
//     RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error)
//     Typed request in, typed response out, plain context.Context.
//
// NewStrictHandler(handler, nil) bridges the two: it returns a value that
// satisfies ServerInterface by wrapping our StrictServerInterface impl.
// Its generated RegisterCustomer(ctx echo.Context) does ctx.Bind(&body) to
// build our typed request, calls our strict method, then calls
// response.VisitRegisterCustomerResponse(ctx.Response()) to write the
// status code + JSON body we returned.
//
// So: Echo only ever talks to ServerInterface. We only ever write
// StrictServerInterface. NewStrictHandler is the adapter in between.


func (h *Handler) RegisterCustomer(ctx context.Context, request RegisterCustomerRequestObject) (RegisterCustomerResponseObject, error) {
	uuid := uuid.New()
	return RegisterCustomer201JSONResponse{
		CustomerUuid: uuid,
	}, nil
}

// Register is the seam between two things that shouldn't know about each
// other's internals: the module's own init sequence (orders.Module.RegisterHttp,
// called from svc.go's phase-4 loop over all modules) and the oapi-codegen-specific
// wiring (RegisterHandlers/NewStrictHandler).
//
// orders/module.go's RegisterHttp just calls Register and trusts this package to
// know how its own generated code works — it never touches RegisterHandlers or
// NewStrictHandler directly. Without this function, RegisterHttp would have to
// import oapi-codegen wiring itself, mixing "which modules exist and in what
// order they're wired" (svc.go's job) with "how this module's HTTP routes attach
// to Echo" (this package's job).
//
// That's also why the signature only takes common.EchoRouter and a Handler:
// everything oapi-codegen-specific stays inside the function body, invisible
// to whoever calls it.
func Register(ctx context.Context, e common.EchoRouter, handler Handler) error {
	RegisterHandlers(e, NewStrictHandler(&handler, nil))
	return nil
}
