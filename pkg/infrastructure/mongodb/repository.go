package mongodb

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const defaultIDField = "_id"

type mongoRepository struct {
	collection *mongo.Collection
	client     *mongo.Client
	logger     *log.Logger
	tracer     trace.TracerProvider
	idField    string
	sortField  string
}

// NewMongoRepository creates a repository for the given collection.
// idField defaults to "_id" when empty; sortField defaults to "created_at" when empty.
func NewMongoRepository(
	mc MongoClient,
	collectionName string,
	logger *log.Logger,
	tracer trace.TracerProvider,
	idField string,
	sortField string,
) Repository {
	if idField == "" {
		idField = defaultIDField
	}
	if sortField == "" {
		sortField = "created_at"
	}
	return &mongoRepository{
		collection: mc.Collection(collectionName),
		client:     mc.Client(),
		logger:     logger,
		tracer:     tracer,
		idField:    idField,
		sortField:  sortField,
	}
}

func (r *mongoRepository) Collection() *mongo.Collection {
	return r.collection
}

func (r *mongoRepository) GetAll(ctx context.Context, target interface{}) error {
	return r.find(ctx, "repository.get-all", target, bson.M{}, nil, 0, 0)
}

func (r *mongoRepository) GetBatch(ctx context.Context, target interface{}, limit, offset int) error {
	return r.find(ctx, "repository.get-batch", target, bson.M{}, nil, limit, offset)
}

func (r *mongoRepository) GetByFilter(ctx context.Context, target interface{}, filter bson.M) error {
	return r.find(ctx, "repository.get-by-filter", target, filter, nil, 0, 0)
}

func (r *mongoRepository) GetByFilterPaging(ctx context.Context, target interface{}, filter bson.M, limit, offset int) error {
	return r.find(ctx, "repository.get-by-filter-paging", target, filter, nil, limit, offset)
}

func (r *mongoRepository) GetByField(ctx context.Context, target interface{}, field string, value interface{}) error {
	return r.GetByFilter(ctx, target, bson.M{field: value})
}

func (r *mongoRepository) GetByFields(ctx context.Context, target interface{}, filters map[string]interface{}) error {
	return r.GetByFilter(ctx, target, mapToBSON(filters))
}

func (r *mongoRepository) GetByFieldPaging(ctx context.Context, target interface{}, field string, value interface{}, limit, offset int) error {
	return r.GetByFilterPaging(ctx, target, bson.M{field: value}, limit, offset)
}

func (r *mongoRepository) GetByFieldsPaging(ctx context.Context, target interface{}, filters map[string]interface{}, limit, offset int) error {
	return r.GetByFilterPaging(ctx, target, mapToBSON(filters), limit, offset)
}

func (r *mongoRepository) GetOneByFilter(ctx context.Context, target interface{}, filter bson.M) error {
	ctx, span := r.trace(ctx, "repository.get-one-by-filter")
	if span != nil {
		defer span.End()
		r.setFilterAttrs(span, filter)
	}

	err := r.collection.FindOne(ctx, filter).Decode(target)
	if err == mongo.ErrNoDocuments {
		if span != nil {
			span.SetStatus(codes.Ok, "not found")
		}
		return ErrNotFound
	}
	if err != nil {
		return r.handleError(span, err)
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (r *mongoRepository) GetOneByField(ctx context.Context, target interface{}, field string, value interface{}) error {
	return r.GetOneByFilter(ctx, target, bson.M{field: value})
}

func (r *mongoRepository) GetOneByFields(ctx context.Context, target interface{}, filters map[string]interface{}) error {
	return r.GetOneByFilter(ctx, target, mapToBSON(filters))
}

func (r *mongoRepository) GetOneByID(ctx context.Context, target interface{}, id interface{}) error {
	return r.GetOneByFilter(ctx, target, bson.M{r.idField: id})
}

func (r *mongoRepository) ExistsByField(ctx context.Context, field string, value interface{}) (bool, error) {
	return r.ExistsByFields(ctx, map[string]interface{}{field: value})
}

func (r *mongoRepository) ExistsByFields(ctx context.Context, filters map[string]interface{}) (bool, error) {
	count, err := r.Count(ctx, mapToBSON(filters))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *mongoRepository) Create(ctx context.Context, document interface{}) error {
	ctx, span := r.trace(ctx, "repository.create")
	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.String("mongo.entity", fmt.Sprintf("%T", document)))
	}

	_, err := r.collection.InsertOne(ctx, document)
	if err != nil {
		return r.handleError(span, err)
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (r *mongoRepository) CreateMany(ctx context.Context, documents []interface{}) error {
	ctx, span := r.trace(ctx, "repository.create-many")
	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int("mongo.count", len(documents)))
	}

	if len(documents) == 0 {
		return nil
	}

	_, err := r.collection.InsertMany(ctx, documents)
	if err != nil {
		return r.handleError(span, err)
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (r *mongoRepository) ReplaceOne(ctx context.Context, filter bson.M, document interface{}) error {
	ctx, span := r.trace(ctx, "repository.replace-one")
	if span != nil {
		defer span.End()
		r.setFilterAttrs(span, filter)
	}

	res, err := r.collection.ReplaceOne(ctx, filter, document)
	if err != nil {
		return r.handleError(span, err)
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (r *mongoRepository) UpdateByFilter(ctx context.Context, filter bson.M, update bson.M) error {
	ctx, span := r.trace(ctx, "repository.update-by-filter")
	if span != nil {
		defer span.End()
		r.setFilterAttrs(span, filter)
	}

	res, err := r.collection.UpdateMany(ctx, filter, wrapUpdate(update))
	if err != nil {
		return r.handleError(span, err)
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}

	if span != nil {
		span.SetAttributes(attribute.Int64("mongo.matched", res.MatchedCount))
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (r *mongoRepository) UpdateOneByID(ctx context.Context, id interface{}, update bson.M) error {
	return r.UpdateByFilter(ctx, bson.M{r.idField: id}, update)
}

func (r *mongoRepository) DeleteByFilter(ctx context.Context, filter bson.M) error {
	ctx, span := r.trace(ctx, "repository.delete-by-filter")
	if span != nil {
		defer span.End()
		r.setFilterAttrs(span, filter)
	}

	res, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return r.handleError(span, err)
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}

	if span != nil {
		span.SetAttributes(attribute.Int64("mongo.deleted", res.DeletedCount))
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (r *mongoRepository) DeleteByID(ctx context.Context, id interface{}) error {
	return r.DeleteByFilter(ctx, bson.M{r.idField: id})
}

func (r *mongoRepository) DeleteByFields(ctx context.Context, filters map[string]interface{}) error {
	return r.DeleteByFilter(ctx, mapToBSON(filters))
}

func (r *mongoRepository) Count(ctx context.Context, filter bson.M) (int64, error) {
	ctx, span := r.trace(ctx, "repository.count")
	if span != nil {
		defer span.End()
		r.setFilterAttrs(span, filter)
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, r.handleError(span, err)
	}

	if span != nil {
		span.SetAttributes(attribute.Int64("mongo.count", count))
		span.SetStatus(codes.Ok, "success")
	}
	return count, nil
}

func (r *mongoRepository) find(
	ctx context.Context,
	spanName string,
	target interface{},
	filter bson.M,
	projection bson.M,
	limit, offset int,
) error {
	ctx, span := r.trace(ctx, spanName)
	if span != nil {
		defer span.End()
		r.setFilterAttrs(span, filter)
		if limit > 0 {
			span.SetAttributes(attribute.Int("mongo.limit", limit))
		}
		if offset > 0 {
			span.SetAttributes(attribute.Int("mongo.offset", offset))
		}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: r.sortField, Value: -1}})

	if limit > 0 {
		opts.SetLimit(int64(limit))
	}
	if offset > 0 {
		opts.SetSkip(int64(offset))
	}
	if projection != nil {
		opts.SetProjection(projection)
	}

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return r.handleError(span, err)
	}
	defer cursor.Close(ctx)

	if err := cursor.All(ctx, target); err != nil {
		return r.handleError(span, err)
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}

func (r *mongoRepository) trace(ctx context.Context, name string) (context.Context, trace.Span) {
	parentSpan := trace.SpanFromContext(ctx)
	if parentSpan == nil || !parentSpan.SpanContext().IsValid() {
		return ctx, nil
	}

	tracer := r.tracer.Tracer("mongodb.repository")
	ctx, span := tracer.Start(ctx, name)
	span.SetAttributes(attribute.String("mongo.collection", r.collection.Name()))
	return ctx, span
}

func (r *mongoRepository) setFilterAttrs(span trace.Span, filter bson.M) {
	if span == nil {
		return
	}
	span.SetAttributes(attribute.String("mongo.filter", fmt.Sprintf("%v", filter)))
}

func (r *mongoRepository) handleError(span trace.Span, err error) error {
	if err == nil {
		return nil
	}
	if r.logger != nil {
		r.logger.WithError(err).Error("mongodb repository error")
	}
	if span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return fmt.Errorf("mongodb: %w", err)
}

func mapToBSON(filters map[string]interface{}) bson.M {
	if filters == nil {
		return bson.M{}
	}
	m := make(bson.M, len(filters))
	for k, v := range filters {
		m[k] = v
	}
	return m
}

// wrapUpdate wraps a plain field map in $set when no update operator is present.
func wrapUpdate(update bson.M) bson.M {
	if update == nil {
		return bson.M{}
	}
	for k := range update {
		if len(k) > 0 && k[0] == '$' {
			return update
		}
	}
	return bson.M{"$set": update}
}

// sessionRepository adds transaction support.
type sessionRepository struct {
	*mongoRepository
}

// NewSessionRepository returns a repository that supports multi-document transactions.
func NewSessionRepository(
	mc MongoClient,
	collectionName string,
	logger *log.Logger,
	tracer trace.TracerProvider,
	idField string,
	sortField string,
) SessionRepository {
	base := NewMongoRepository(mc, collectionName, logger, tracer, idField, sortField).(*mongoRepository)
	return &sessionRepository{mongoRepository: base}
}

func (r *sessionRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	ctx, span := r.trace(ctx, "repository.with-transaction")
	if span != nil {
		defer span.End()
	}

	session, err := r.client.StartSession()
	if err != nil {
		return r.handleError(span, err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, fn(sessCtx)
	})
	if err != nil {
		return r.handleError(span, err)
	}

	if span != nil {
		span.SetStatus(codes.Ok, "success")
	}
	return nil
}
