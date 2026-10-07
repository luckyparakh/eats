package orders

import (
	"context"
	"embed"

	"github.com/jackc/pgx/v5/pgxpool"

	"eats/backend/common"
	"eats/backend/common/module"
	"eats/backend/common/module/contracts"
	"eats/backend/orders/adapters/db"
	http2 "eats/backend/orders/api/http"
	ordersModule "eats/backend/orders/api/module"
	"eats/backend/orders/app"
)

// Module is the composition root for the orders module: it wires this module's own
// adapters, handlers, and dependencies together. It is not the app's entry point
// (see backend/cmd/main.go) — the framework calls into Name/Init/RegisterContracts/
// RegisterHttp. Because it sits above the module's sub-packages, it's the one place
// allowed to import across them freely; e.g. it imports both http and db to inject the
// db-based repository into the http handler, which individual sub-packages must not do
// directly (http importing db would create the cyclic import the repository pattern avoids).
type Module struct {
	pgxDb       *pgxpool.Pool
	httpHandler http2.Handler

	modules *contracts.Contracts
}

func NewModule(pgxDb *pgxpool.Pool, modules *contracts.Contracts) *Module {
	return &Module{
		pgxDb:   pgxDb,
		modules: modules,
	}
}

func (m *Module) Name() module.Name {
	return "orders"
}

//go:embed adapters/db/migrations/*.sql
var embedMigrations embed.FS

func (m *Module) Init(ctx context.Context) error {
	cr := db.NewCustomerRepository(m.pgxDb)
	svc := app.NewService(cr, struct{}{})
	httpHandler := http2.NewHandler(svc)
	m.httpHandler = httpHandler

	if err := common.MigrateDatabaseUp(
		ctx,
		string(m.Name()),
		m.pgxDb,
		embedMigrations,
		"adapters/db/migrations",
	); err != nil {
		return err
	}

	return nil
}

func (m *Module) RegisterContracts(ctx context.Context, contracts *contracts.Contracts) error {
	contracts.Orders = ordersModule.Orders{}
	return nil
}

func (m *Module) RegisterHttp(ctx context.Context, e common.EchoRouter) error {
	return http2.Register(ctx, e, m.httpHandler)
}
