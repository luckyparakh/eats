package app

import (
	"context"
	"strings"

	"eats/backend/common"
	"eats/backend/common/shared"
)

type CustomerUUID struct {
	common.UUID
}

type Customer struct {
	CustomerUUID CustomerUUID
	Name         string
	Email        string
	Address      shared.Address
	PhoneNumber  string
}

type CustomerRepository interface {
	RegisterCustomer(ctx context.Context, customer Customer) error
}

func (s *Service) RegisterCustomer(ctx context.Context, customer Customer) error {
	errorDetails := make([]common.ErrorDetails, 0)
	if customer.CustomerUUID.UUID.IsZero() {
		errorDetails = append(errorDetails,
			common.ErrorDetails{
				EntityType: "customer",
				EntityID:   "",
				ErrorSlug:  "empty-uuid",
				Message:    "UUID can't be empty",
			},
		)
	}
	if strings.TrimSpace(customer.Name) == "" {
		errorDetails = append(errorDetails,
			common.ErrorDetails{
				EntityType: "customer",
				EntityID:   customer.CustomerUUID.UUID.String(),
				ErrorSlug:  "empty-name",
				Message:    "Name can't be empty",
			},
		)
	}
	if strings.TrimSpace(customer.Email) == "" {
		errorDetails = append(errorDetails,
			common.ErrorDetails{
				EntityType: "customer",
				EntityID:   customer.CustomerUUID.UUID.String(),
				ErrorSlug:  "empty-email",
				Message:    "Email can't be empty",
			},
		)
	}
	if strings.TrimSpace(customer.PhoneNumber) == "" {
		errorDetails = append(errorDetails,
			common.ErrorDetails{
				EntityType: "customer",
				EntityID:   customer.CustomerUUID.UUID.String(),
				ErrorSlug:  "empty-phone-number",
				Message:    "Number can't be empty",
			},
		)
	}
	if customer.Address.IsZero() {
		errorDetails = append(errorDetails,
			common.ErrorDetails{
				EntityType: "customer",
				EntityID:   customer.CustomerUUID.UUID.String(),
				ErrorSlug:  "empty-address",
				Message:    "Address can't be empty",
			},
		)
	}
	if len(errorDetails) > 0 {
		return common.NewInvalidInputError(
			"invalid_customer_data",
			"Invalid customer data",
		).WithDetails(errorDetails)
	}
	return s.customerRepository.RegisterCustomer(ctx, customer)
}
