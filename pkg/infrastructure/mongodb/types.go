package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Repository provides generic CRUD operations for a MongoDB collection.
type Repository interface {
	Collection() *mongo.Collection

	GetAll(ctx context.Context, target interface{}) error
	GetBatch(ctx context.Context, target interface{}, limit, offset int) error

	GetByFilter(ctx context.Context, target interface{}, filter bson.M) error
	GetByFilterPaging(ctx context.Context, target interface{}, filter bson.M, limit, offset int) error
	GetOneByFilter(ctx context.Context, target interface{}, filter bson.M) error

	GetByField(ctx context.Context, target interface{}, field string, value interface{}) error
	GetByFields(ctx context.Context, target interface{}, filters map[string]interface{}) error
	GetByFieldPaging(ctx context.Context, target interface{}, field string, value interface{}, limit, offset int) error
	GetByFieldsPaging(ctx context.Context, target interface{}, filters map[string]interface{}, limit, offset int) error

	GetOneByField(ctx context.Context, target interface{}, field string, value interface{}) error
	GetOneByFields(ctx context.Context, target interface{}, filters map[string]interface{}) error
	GetOneByID(ctx context.Context, target interface{}, id interface{}) error

	ExistsByField(ctx context.Context, field string, value interface{}) (bool, error)
	ExistsByFields(ctx context.Context, filters map[string]interface{}) (bool, error)

	Create(ctx context.Context, document interface{}) error
	CreateMany(ctx context.Context, documents []interface{}) error
	ReplaceOne(ctx context.Context, filter bson.M, document interface{}) error
	UpdateByFilter(ctx context.Context, filter bson.M, update bson.M) error
	UpdateOneByID(ctx context.Context, id interface{}, update bson.M) error

	DeleteByFilter(ctx context.Context, filter bson.M) error
	DeleteByID(ctx context.Context, id interface{}) error
	DeleteByFields(ctx context.Context, filters map[string]interface{}) error

	Count(ctx context.Context, filter bson.M) (int64, error)
}

// SessionRepository extends Repository with multi-document transaction support.
type SessionRepository interface {
	Repository
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
