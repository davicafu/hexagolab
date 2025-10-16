// en internal/infra/db/mongodb/outbox_repository.go
package mongodb

import (
	"context"
	"fmt"
	"time"

	sharedDomain "github.com/davicafu/hexagolab/shared/domain"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// OutboxRepoMongoDB implementa la interfaz sharedDomain.OutboxRepository.
type OutboxRepoMongoDB struct {
	outboxColl *mongo.Collection
}

func NewOutboxRepoMongoDB(client *mongo.Client, dbName string) *OutboxRepoMongoDB {
	outboxColl := client.Database(dbName).Collection("outbox")
	return &OutboxRepoMongoDB{outboxColl: outboxColl}
}

// mongoOutboxEvent es un helper para mapear los documentos de la base de datos a un struct.
type mongoOutboxEvent struct {
	ID            uuid.UUID   `bson:"_id"`
	AggregateType string      `bson:"aggregateType"`
	AggregateID   string      `bson:"aggregateId"`
	EventType     string      `bson:"eventType"`
	Payload       interface{} `bson:"payload"`
	CreatedAt     time.Time   `bson:"createdAt"`
	Processed     bool        `bson:"processed"`
}

// FetchPendingOutbox obtiene los eventos no procesados de la colección outbox.
func (r *OutboxRepoMongoDB) FetchPendingOutbox(ctx context.Context, limit int) ([]sharedDomain.OutboxEvent, error) {
	// Filtro para buscar documentos no procesados.
	filter := bson.M{"processed": false}

	// Opciones para ordenar por fecha y limitar el número de documentos.
	opts := options.Find().SetSort(bson.D{{"createdAt", 1}}).SetLimit(int64(limit))

	cursor, err := r.outboxColl.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var events []sharedDomain.OutboxEvent
	for cursor.Next(ctx) {
		var mo mongoOutboxEvent
		if err := cursor.Decode(&mo); err != nil {
			return nil, err
		}
		// Convertimos el struct BSON a nuestro struct de dominio.
		events = append(events, fromMongoOutboxEvent(&mo))
	}

	return events, nil
}

// MarkOutboxProcessed marca un evento como procesado.
func (r *OutboxRepoMongoDB) MarkOutboxProcessed(ctx context.Context, id uuid.UUID) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"processed": true}}

	res, err := r.outboxColl.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("outbox event not found: %s", id)
	}

	return nil
}

// fromMongoOutboxEvent es un helper para convertir de BSON a nuestro tipo de dominio.
func fromMongoOutboxEvent(mo *mongoOutboxEvent) sharedDomain.OutboxEvent {
	return sharedDomain.OutboxEvent{
		ID:            mo.ID,
		AggregateType: mo.AggregateType,
		AggregateID:   mo.AggregateID,
		EventType:     mo.EventType,
		Payload:       mo.Payload,
		CreatedAt:     mo.CreatedAt,
		Processed:     mo.Processed,
	}
}

// Verificación en tiempo de compilación.
var _ sharedDomain.OutboxRepository = (*OutboxRepoMongoDB)(nil)
