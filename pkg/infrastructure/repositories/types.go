package repositories

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// Repository is a generic DB handler that cares about default error handling
type Repository interface {
	GetAll(ctx context.Context, target interface{}, preloads ...string) error
	GetBatch(ctx context.Context, target interface{}, limit, offset int, preloads ...string) error

	GetWhere(ctx context.Context, target interface{}, condition string, preloads ...string) error
	GetWhereWithArgs(ctx context.Context, target interface{}, condition string, args []any, preloads ...string) error
	GetWherePagging(ctx context.Context, target interface{}, condition string, limit, offset int, preloads ...string) error
	GetWhereWithOrder(ctx context.Context, target interface{}, condition string, orderBy string, limit, offset int, preloads []string, args ...interface{}) error
	GetWhereBatch(ctx context.Context, target interface{}, condition string, limit, offset int, preloads ...string) error

	GetByField(ctx context.Context, target interface{}, field string, value interface{}, preloads ...string) error
	GetByFields(ctx context.Context, target interface{}, filters map[string]interface{}, preloads ...string) error
	GetByFieldBatch(ctx context.Context, target interface{}, field string, value interface{}, limit, offset int, preloads ...string) error
	GetByFieldsBatch(ctx context.Context, target interface{}, filters map[string]interface{}, limit, offset int, preloads ...string) error

	GetOneByField(ctx context.Context, target interface{}, field string, value interface{}, preloads ...string) error
	GetOneByFields(ctx context.Context, target interface{}, filters map[string]interface{}, preloads ...string) error

	ExistsByField(ctx context.Context, target interface{}, field string, value interface{}) (bool, error)
	ExistsByFields(ctx context.Context, target interface{}, filters map[string]interface{}) (bool, error)

	// GetOneByID assumes you have a PK column "id" which is a UUID. If this is not the case just ignore the method
	// and add a custom struct with this Repository embedded.
	GetOneByID(ctx context.Context, target interface{}, id any, preloads ...string) error

	Create(ctx context.Context, target interface{}) error
	Save(ctx context.Context, target interface{}) error
	Update(ctx context.Context, target interface{}, updates map[string]interface{}, condition string, args ...interface{}) error
	Delete(ctx context.Context, target interface{}) error
	DeleteByFields(ctx context.Context, target interface{}, filters map[string]interface{}) error
	DeleteWhere(ctx context.Context, target interface{}, condition string, args ...interface{}) error
	DeleteWhereTx(ctx context.Context, target interface{}, condition string, tx *gorm.DB, args ...interface{}) error

	DB() *gorm.DB
	DBWithPreloads(preloads []string) *gorm.DB
	HandleError(ctx context.Context, res *gorm.DB, span trace.Span) error
	HandleOneError(ctx context.Context, res *gorm.DB, span trace.Span) error

	// Count
	Count(ctx context.Context, model interface{}, filters map[string]interface{}) (int64, error)
	CountWithWhere(ctx context.Context, model interface{}, condition string, args ...interface{}) (int64, error)
	CountWithJoin(ctx context.Context, model interface{}, join string, where map[string]interface{}) (int64, error)

	// SQL
	RawQuery(ctx context.Context, target interface{}, sql string, args ...interface{}) error
	ExecSQL(ctx context.Context, sql string, args ...interface{}) error

	//Trace - unexported method, not part of public interface
	// trace(ctx context.Context, name string) (context.Context, trace.Span)
}

// TransactionRepository extends Repository with modifier functions that accept a transaction
type TransactionRepository interface {
	Repository
	BeginTx(ctx context.Context) (*gorm.DB, error)
	CreateTx(ctx context.Context, target interface{}, tx *gorm.DB) error
	UpdateTx(ctx context.Context, target interface{}, updates map[string]interface{}, tx *gorm.DB) error
	UpdateWithConditionTx(ctx context.Context, target interface{}, updates map[string]interface{}, condition string, tx *gorm.DB, args ...interface{}) error
	DeleteTx(ctx context.Context, target interface{}, tx *gorm.DB) error
	SaveTx(ctx context.Context, target interface{}, tx *gorm.DB) error
	CommitTx(ctx context.Context, tx *gorm.DB) error
}
