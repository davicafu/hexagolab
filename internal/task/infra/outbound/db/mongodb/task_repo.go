// en internal/task/infra/outbound/db/mongodb/task_repo_mongodb.go
package mongodb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	// --- Importaciones del dominio y compartidas ---
	sharedDomain "github.com/davicafu/hexagolab/internal/shared/domain"
	sharedQuery "github.com/davicafu/hexagolab/internal/shared/infra/platform/query"
	taskDomain "github.com/davicafu/hexagolab/internal/task/domain"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// TaskRepoMongoDB implementa la interfaz TaskRepository para MongoDB.
type TaskRepoMongoDB struct {
	client     *mongo.Client
	dbName     string
	tasksColl  *mongo.Collection
	outboxColl *mongo.Collection
}

// NewTaskRepoMongoDB es el constructor del repositorio.
func NewTaskRepoMongoDB(ctx context.Context, client *mongo.Client, dbName string) (*TaskRepoMongoDB, error) {
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("could not ping mongoDB: %w", err)
	}

	db := client.Database(dbName)
	return &TaskRepoMongoDB{
		client:     client,
		dbName:     dbName,
		tasksColl:  db.Collection("tasks"),
		outboxColl: db.Collection("outbox"),
	}, nil
}

// --- Structs de BSON para el mapeo ---
// Se definen localmente para no "contaminar" el dominio con tags de BSON.

type mongoTask struct {
	ID          uuid.UUID             `bson:"_id"`
	Title       string                `bson:"title"`
	Description string                `bson:"description"`
	AssigneeID  uuid.UUID             `bson:"assigneeId"`
	Status      taskDomain.TaskStatus `bson:"status"`
	CreatedAt   time.Time             `bson:"createdAt"`
	UpdatedAt   time.Time             `bson:"updatedAt"`
}

type mongoOutboxEvent struct {
	ID            uuid.UUID   `bson:"_id"`
	AggregateType string      `bson:"aggregateType"`
	AggregateID   string      `bson:"aggregateId"`
	EventType     string      `bson:"eventType"`
	Payload       interface{} `bson:"payload"`
	CreatedAt     time.Time   `bson:"createdAt"`
	Processed     bool        `bson:"processed"`
}

// --- CRUD Transaccional ---

func (r *TaskRepoMongoDB) Create(ctx context.Context, t *taskDomain.Task, evt sharedDomain.OutboxEvent) error {
	session, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	// La transacción asegura que ambas inserciones (tarea y evento) sean atómicas.
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// 1. Insertar la tarea
		mt := toMongoTask(t)
		if _, err := r.tasksColl.InsertOne(sessCtx, mt); err != nil {
			return nil, err
		}
		// 2. Insertar el evento de outbox
		mo := toMongoOutboxEvent(evt)
		if _, err := r.outboxColl.InsertOne(sessCtx, mo); err != nil {
			return nil, err
		}
		return nil, nil
	})

	return err
}

func (r *TaskRepoMongoDB) Update(ctx context.Context, t *taskDomain.Task, evt sharedDomain.OutboxEvent) error {
	session, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		mt := toMongoTask(t)
		filter := bson.M{"_id": mt.ID}
		update := bson.M{"$set": mt}

		res, err := r.tasksColl.UpdateOne(sessCtx, filter, update)
		if err != nil {
			return nil, err
		}
		if res.MatchedCount == 0 {
			return nil, taskDomain.ErrTaskNotFound
		}

		mo := toMongoOutboxEvent(evt)
		if _, err := r.outboxColl.InsertOne(sessCtx, mo); err != nil {
			return nil, err
		}

		return nil, nil
	})

	return err
}

func (r *TaskRepoMongoDB) DeleteByID(ctx context.Context, id uuid.UUID, evt sharedDomain.OutboxEvent) error {
	session, err := r.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		res, err := r.tasksColl.DeleteOne(sessCtx, bson.M{"_id": id})
		if err != nil {
			return nil, err
		}
		if res.DeletedCount == 0 {
			return nil, taskDomain.ErrTaskNotFound
		}

		mo := toMongoOutboxEvent(evt)
		if _, err := r.outboxColl.InsertOne(sessCtx, mo); err != nil {
			return nil, err
		}

		return nil, nil
	})

	return err
}

// --- Lectura ---

func (r *TaskRepoMongoDB) GetByID(ctx context.Context, id uuid.UUID) (*taskDomain.Task, error) {
	var mt mongoTask
	err := r.tasksColl.FindOne(ctx, bson.M{"_id": id}).Decode(&mt)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, taskDomain.ErrTaskNotFound
		}
		return nil, err
	}
	return fromMongoTask(&mt), nil
}

func (r *TaskRepoMongoDB) ListByCriteria(ctx context.Context, criteria sharedDomain.Criteria, pagination sharedQuery.Pagination, sort sharedQuery.Sort) ([]*taskDomain.Task, error) {
	filter := criteriaToMongoFilter(criteria)
	opts := options.Find()

	// Paginación
	if p, ok := pagination.(sharedQuery.OffsetPagination); ok {
		opts.SetSkip(int64(p.Offset))
		opts.SetLimit(int64(p.Limit))
	}

	// Ordenamiento
	if sort.Field != "" {
		sortDir := 1 // Ascendente por defecto
		if sort.Desc {
			sortDir = -1 // Descendente
		}
		opts.SetSort(bson.D{{Key: sort.Field, Value: sortDir}})
	}

	cursor, err := r.tasksColl.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []*taskDomain.Task
	for cursor.Next(ctx) {
		var mt mongoTask
		if err := cursor.Decode(&mt); err != nil {
			return nil, err
		}
		tasks = append(tasks, fromMongoTask(&mt))
	}

	return tasks, nil
}

// --- Helpers de Mapeo y Conversión ---

func toMongoTask(t *taskDomain.Task) *mongoTask {
	return &mongoTask{
		ID: t.ID, Title: t.Title, Description: t.Description,
		AssigneeID: t.AssigneeID, Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func fromMongoTask(mt *mongoTask) *taskDomain.Task {
	return &taskDomain.Task{
		ID: mt.ID, Title: mt.Title, Description: mt.Description,
		AssigneeID: mt.AssigneeID, Status: mt.Status, CreatedAt: mt.CreatedAt, UpdatedAt: mt.UpdatedAt,
	}
}

func toMongoOutboxEvent(evt sharedDomain.OutboxEvent) *mongoOutboxEvent {
	return &mongoOutboxEvent{
		ID: evt.ID, AggregateType: evt.AggregateType, AggregateID: evt.AggregateID,
		EventType: evt.EventType, Payload: evt.Payload, CreatedAt: evt.CreatedAt, Processed: false,
	}
}

func criteriaToMongoFilter(criteria sharedDomain.Criteria) bson.D {
	if criteria == nil {
		return bson.D{}
	}
	conds := criteria.ToConditions()
	if len(conds) == 0 {
		return bson.D{}
	}

	filter := bson.D{}
	for _, c := range conds {
		// Mapeo de operadores genéricos a operadores de MongoDB
		var mongoOp string
		switch c.Op {
		case sharedDomain.OpEq:
			mongoOp = "$eq"
		case sharedDomain.OpGt:
			mongoOp = "$gt"
		case sharedDomain.OpGte:
			mongoOp = "$gte"
		case sharedDomain.OpLt:
			mongoOp = "$lt"
		case sharedDomain.OpLte:
			mongoOp = "$lte"
		case sharedDomain.OpLike, sharedDomain.OpILike:
			mongoOp = "$regex"
		default:
			mongoOp = "$eq" // Operador por defecto
		}

		// Para ILIKE, añadimos la opción 'i' de insensibilidad a mayúsculas
		if c.Op == sharedDomain.OpILike {
			filter = append(filter, bson.E{Key: c.Field, Value: bson.M{mongoOp: strings.Trim(c.Value.(string), "%"), "$options": "i"}})
		} else {
			filter = append(filter, bson.E{Key: c.Field, Value: bson.M{mongoOp: c.Value}})
		}
	}
	return filter
}
