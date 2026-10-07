package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"eats/backend/orders/adapters/db/dbmodels"
	"eats/backend/orders/app"
)

type CustomerRepository struct {
	db *pgxpool.Pool
}

func NewCustomerRepository(db *pgxpool.Pool) *CustomerRepository {
	if db == nil {
		panic("db connection pool cannot be nil")
	}

	return &CustomerRepository{
		db: db,
	}
}

func (r *CustomerRepository) RegisterCustomer(ctx context.Context, customer app.Customer) error {
	queries := dbmodels.New(r.db)

	args := dbmodels.InsertCustomerParams{
		CustomerUuid: app.CustomerUUID{UUID: customer.CustomerUUID.UUID},
		Name:         customer.Name,
		Email:        string(customer.Email),
		Address:      customer.Address,
		PhoneNumber:  customer.PhoneNumber,
	}
	queries.InsertCustomer(ctx, args)
	return nil
}
