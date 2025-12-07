package repositories

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"gorm.io/gorm"
)

type gormRepository struct {
	logger       *log.Logger
	db           *gorm.DB
	tracer       trace.TracerProvider
	defaultJoins []string
}

func NewGormRepository(db *gorm.DB, logger *log.Logger, tracer trace.TracerProvider, defaultJoins ...string) TransactionRepository {
	return &gormRepository{
		defaultJoins: defaultJoins,
		logger:       logger,
		db:           db,
		tracer:       tracer,
	}
}

func (r *gormRepository) DB() *gorm.DB {
	return r.DBWithPreloads(nil)
}

func (r *gormRepository) GetAll(ctx context.Context, target interface{}, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-all")
	if span != nil {
		defer span.End()
	}

	res := r.DBWithPreloads(preloads).
		Unscoped().
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetBatch(ctx context.Context, target interface{}, limit, offset int, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-batch")
	if span != nil {
		defer span.End()
	}

	res := r.DBWithPreloads(preloads).
		Unscoped().
		Limit(limit).
		Offset(offset).
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetWhere(ctx context.Context, target interface{}, condition string, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.getWhere")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.condition", condition),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(condition).
		Order("created_at DESC").
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetWhereWithArgs(ctx context.Context, target interface{}, condition string, args []any, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-where-with-args")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.condition", condition),
			attribute.String("gorm.args", fmt.Sprintf("%+v", args)),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(condition, args...).
		Order("created_at DESC").
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetWherePagging(ctx context.Context, target interface{}, condition string, limit, offset int, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-where-pagging")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.condition", condition),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(condition).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetWhereBatch(ctx context.Context, target interface{}, condition string, limit, offset int, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-where-batch")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.condition", condition),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(condition).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetByField(ctx context.Context, target interface{}, field string, value interface{}, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-by-field")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.field", field),
			attribute.String("gorm.value", fmt.Sprintf("%v", value)),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(fmt.Sprintf("%v = ?", field), value).
		Order("created_at DESC").
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetByFields(ctx context.Context, target interface{}, filters map[string]interface{}, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-by-fields")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.filters", fmt.Sprintf("%+v", filters)),
		)
	}

	db := r.DBWithPreloads(preloads).WithContext(ctx)
	for field, value := range filters {
		db = db.Where(fmt.Sprintf("%v = ?", field), value)
	}

	res := db.Order("created_at DESC").Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetByFieldBatch(ctx context.Context, target interface{}, field string, value interface{}, limit, offset int, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-by-field-batch")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.field", field),
			attribute.String("gorm.value", fmt.Sprintf("%v", value)),
			attribute.Int("gorm.limit", limit),
			attribute.Int("gorm.offset", offset),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(fmt.Sprintf("%v = ?", field), value).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetByFieldsBatch(ctx context.Context, target interface{}, filters map[string]interface{}, limit, offset int, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-by-fields-batch")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.filters", fmt.Sprintf("%+v", filters)),
		)
	}

	db := r.DBWithPreloads(preloads)
	for field, value := range filters {
		db = db.Where(fmt.Sprintf("%v = ?", field), value)
	}

	res := db.WithContext(ctx).
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) GetOneByField(ctx context.Context, target interface{}, field string, value interface{}, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-one-by-field")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.field", field),
			attribute.String("gorm.value", fmt.Sprintf("%v", value)),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(fmt.Sprintf("%v = ?", field), value).
		Order("created_at DESC").
		First(target)

	return r.HandleOneError(ctx, res, span)
}

func (r *gormRepository) GetOneByFields(ctx context.Context, target interface{}, filters map[string]interface{}, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-one-by-fields")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.filters", fmt.Sprintf("%+v", filters)),
		)
	}

	db := r.DBWithPreloads(preloads).WithContext(ctx)
	for field, value := range filters {
		db = db.Where(fmt.Sprintf("%v = ?", field), value)
	}

	res := db.Order("created_at DESC").First(target)
	return r.HandleOneError(ctx, res, span)
}

func (r *gormRepository) GetOneByID(ctx context.Context, target interface{}, id any, preloads ...string) error {
	ctx, span := r.trace(ctx, "repository.get-one-by-id")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.id", fmt.Sprintf("%v", id)),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where("id = ?", id).
		Order("created_at DESC").
		First(target)

	return r.HandleOneError(ctx, res, span)
}

func (r *gormRepository) Create(ctx context.Context, target interface{}) error {
	ctx, span := r.trace(ctx, "repository.create")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
		)
	}

	res := r.db.WithContext(ctx).Create(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) CreateTx(ctx context.Context, target interface{}, tx *gorm.DB) error {
	ctx, span := r.trace(ctx, "repository.create-tx")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
		)
	}

	res := tx.WithContext(ctx).Create(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) Save(ctx context.Context, target interface{}) error {
	ctx, span := r.trace(ctx, "repository.save")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
		)
	}

	res := r.db.WithContext(ctx).Save(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) SaveTx(ctx context.Context, target interface{}, tx *gorm.DB) error {

	ctx, span := r.trace(ctx, "repository.save-tx")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
		)
	}

	res := tx.WithContext(ctx).Save(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) Update(ctx context.Context, target interface{}, updates map[string]interface{}, condition string, args ...interface{}) error {
	ctx, span := r.trace(ctx, "repository.update")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.operation", "update"),
		)
	}

	res := r.db.WithContext(ctx).Model(target).Where(condition, args...).Updates(updates)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) UpdateTx(ctx context.Context, target interface{}, updates map[string]interface{}, tx *gorm.DB) error {
	ctx, span := r.trace(ctx, "repository.update-tx")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.operation", "update"),
		)
	}

	res := tx.WithContext(ctx).Model(target).Updates(updates)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) UpdateWithConditionTx(ctx context.Context, target interface{}, updates map[string]interface{}, condition string, tx *gorm.DB, args ...interface{}) error {
	ctx, span := r.trace(ctx, "repository.update-with-condition-tx")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.operation", "update"),
			attribute.String("gorm.condition", condition),
		)
	}

	res := tx.WithContext(ctx).Model(target).Where(condition, args...).Updates(updates)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) Delete(ctx context.Context, target interface{}) error {
	ctx, span := r.trace(ctx, "repository.delete")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.operation", "delete"),
		)
	}

	res := r.db.WithContext(ctx).Delete(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) DeleteTx(ctx context.Context, target interface{}, tx *gorm.DB) error {
	ctx, span := r.trace(ctx, "repository.delete-tx")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.operation", "delete"),
		)
	}

	res := tx.WithContext(ctx).Delete(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) BeginTx(ctx context.Context) (*gorm.DB, error) {
	ctx, span := r.trace(ctx, "repository.begin-transaction")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.operation", "begin"),
		)
	}

	tx := r.db.Begin()
	if tx.Error != nil {
		if span != nil {
			span.RecordError(tx.Error)
			span.SetStatus(codes.Error, tx.Error.Error())
		}
		return nil, tx.Error
	}

	// Set context for transaction
	tx = tx.WithContext(ctx)

	if span != nil {
		span.SetStatus(codes.Ok, "Transaction begun successfully")
	}

	return tx, nil
}

func (r *gormRepository) CommitTx(ctx context.Context, tx *gorm.DB) error {
	ctx, span := r.trace(ctx, "repository.commit-transaction")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.operation", "commit"),
		)
	}

	if err := tx.WithContext(ctx).Commit().Error; err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return err
	}

	if span != nil {
		span.SetStatus(codes.Ok, "Transaction committed successfully")
	}

	return nil
}

func (r *gormRepository) DeleteByFields(ctx context.Context, target interface{}, filters map[string]interface{}) error {
	ctx, span := r.trace(ctx, "repository.delete-by-fields")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.operation", "delete-by-fields"),
			attribute.String("gorm.filters", fmt.Sprintf("%+v", filters)),
		)
	}

	db := r.db.WithContext(ctx).Model(target)
	for field, value := range filters {
		db = db.Where(fmt.Sprintf("%s = ?", field), value)
	}

	res := db.Delete(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) HandleError(ctx context.Context, res *gorm.DB, span trace.Span) error {

	if res.Error != nil && res.Error != gorm.ErrRecordNotFound {
		err := fmt.Errorf("error: %w", res.Error)
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return err
	}

	return nil
}

func (r *gormRepository) HandleOneError(ctx context.Context, res *gorm.DB, span trace.Span) error {

	if err := r.HandleError(ctx, res, span); err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		return err
	}

	if res.RowsAffected != 1 {
		return ErrNotFound
	}

	return nil
}

func (r *gormRepository) DBWithPreloads(preloads []string) *gorm.DB {
	dbConn := r.db

	for _, join := range r.defaultJoins {
		dbConn = dbConn.Joins(join)
	}

	for _, preload := range preloads {
		dbConn = dbConn.Preload(preload)
	}

	return dbConn
}

func (r *gormRepository) ExistsByField(ctx context.Context, target interface{}, field string, value interface{}) (bool, error) {
	ctx, span := r.trace(ctx, "repository.exists-by-field")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.field", field),
			attribute.String("gorm.value", fmt.Sprintf("%v", value)),
		)
	}

	var count int64
	res := r.db.Model(target).
		WithContext(ctx).
		Where(fmt.Sprintf("%s = ?", field), value).
		Count(&count)

	if res.Error != nil {
		return false, res.Error
	}

	return count > 0, nil
}

func (r *gormRepository) ExistsByFields(ctx context.Context, target interface{}, filters map[string]interface{}) (bool, error) {
	ctx, span := r.trace(ctx, "repository.exists-by-fields")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.filters", fmt.Sprintf("%+v", filters)),
		)
	}

	var count int64
	db := r.db.Model(target)
	for field, value := range filters {
		db = db.Where(fmt.Sprintf("%s = ?", field), value)
	}

	res := db.WithContext(ctx).Count(&count)

	if res.Error != nil {
		return false, res.Error
	}

	return count > 0, nil
}

func (r *gormRepository) Count(ctx context.Context, model interface{}, filters map[string]interface{}) (int64, error) {
	ctx, span := r.trace(ctx, "repository.count")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", model)),
			attribute.String("gorm.filters", fmt.Sprintf("%+v", filters)),
		)
	}

	var count int64
	db := r.db.Model(model)
	for key, value := range filters {
		db = db.Where(key+" = ?", value)
	}
	res := db.WithContext(ctx).Count(&count)
	if res.Error != nil {
		if span != nil {
			span.RecordError(res.Error)
			span.SetStatus(codes.Error, res.Error.Error())
		}
		return 0, res.Error
	}

	if span != nil {
		span.SetAttributes(attribute.Int64("gorm.count", count))
		span.SetStatus(codes.Ok, "Count completed successfully")
	}
	return count, nil
}

func (r *gormRepository) CountWithJoin(ctx context.Context, model interface{}, join string, where map[string]interface{}) (int64, error) {
	ctx, span := r.trace(ctx, "repository.count-with-join")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", model)),
			attribute.String("gorm.join", join),
			attribute.String("gorm.where", fmt.Sprintf("%+v", where)),
		)
	}

	var count int64
	db := r.db.Model(model)
	if join != "" {
		db = db.Joins(join)
	}
	for key, value := range where {
		db = db.Where(key+" = ?", value)
	}
	res := db.WithContext(ctx).Count(&count)
	if res.Error != nil {
		if span != nil {
			span.RecordError(res.Error)
			span.SetStatus(codes.Error, res.Error.Error())
		}
		return 0, res.Error
	}

	if span != nil {
		span.SetAttributes(attribute.Int64("gorm.count", count))
		span.SetStatus(codes.Ok, "Count with join completed successfully")
	}
	return count, nil
}

func (r *gormRepository) CountWithWhere(ctx context.Context, model interface{}, condition string, args ...interface{}) (int64, error) {
	ctx, span := r.trace(ctx, "repository.count-with-where")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", model)),
			attribute.String("gorm.condition", condition),
		)
	}

	var count int64
	res := r.db.Model(model).WithContext(ctx).Where(condition, args...).Count(&count)
	if res.Error != nil {
		if span != nil {
			span.RecordError(res.Error)
			span.SetStatus(codes.Error, res.Error.Error())
		}
		return 0, res.Error
	}

	if span != nil {
		span.SetAttributes(attribute.Int64("gorm.count", count))
		span.SetStatus(codes.Ok, "Count with where completed successfully")
	}
	return count, nil
}

func (r *gormRepository) GetWhereWithOrder(ctx context.Context, target interface{}, condition string, orderBy string, limit, offset int, preloads []string, args ...interface{}) error {
	ctx, span := r.trace(ctx, "repository.get-where-with-order")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.condition", condition),
			attribute.String("gorm.orderBy", orderBy),
		)
	}

	res := r.DBWithPreloads(preloads).
		WithContext(ctx).
		Where(condition, args...).
		Order(orderBy).
		Limit(limit).
		Offset(offset).
		Find(target)

	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) DeleteWhere(ctx context.Context, target interface{}, condition string, args ...interface{}) error {
	ctx, span := r.trace(ctx, "repository.delete-where")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.condition", condition),
		)
	}

	res := r.db.WithContext(ctx).Where(condition, args...).Delete(target)
	return r.HandleError(ctx, res, span)
}

// DeleteWhereWithTx deletes records based on custom SQL condition within transaction
func (r *gormRepository) DeleteWhereTx(ctx context.Context, target interface{}, condition string, tx *gorm.DB, args ...interface{}) error {
	ctx, span := r.trace(ctx, "repository.delete-where-tx")
	if span != nil {
		defer span.End()
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("gorm.entity", fmt.Sprintf("%T", target)),
			attribute.String("gorm.condition", condition),
		)
	}

	res := tx.WithContext(ctx).Where(condition, args...).Delete(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) RawQuery(ctx context.Context, target interface{}, sql string, args ...interface{}) error {
	ctx, span := r.trace(ctx, "repository.raw-query")
	if span != nil {
		defer span.End()
	}

	res := r.db.WithContext(ctx).Raw(sql, args...).Scan(target)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) ExecSQL(ctx context.Context, sql string, args ...interface{}) error {
	ctx, span := r.trace(ctx, "repository.exec-sql")
	if span != nil {
		defer span.End()
	}

	res := r.db.WithContext(ctx).Exec(sql, args...)
	return r.HandleError(ctx, res, span)
}

func (r *gormRepository) trace(ctx context.Context, name string) (context.Context, trace.Span) {
	// Only create span if there's a parent span in context (i.e., from write operations)
	// This prevents creating orphaned traces for GET requests
	parentSpan := trace.SpanFromContext(ctx)
	if parentSpan == nil || !parentSpan.SpanContext().IsValid() {
		// No parent span, return context without creating new span
		// This prevents orphaned traces - repository operations will be part of parent trace
		return ctx, nil
	}

	// Create child span only when there's a parent span
	tracer := r.tracer.Tracer("gorm.repository")
	ctx, span := tracer.Start(ctx, name)
	return ctx, span
}
