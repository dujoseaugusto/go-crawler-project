package repository

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PropertyRepository define as operações que podem ser realizadas no repositório de propriedades
type PropertyRepository interface {
	Save(ctx context.Context, property Property) error
	FindAll(ctx context.Context) ([]Property, error)
	Close()
}

type Property struct {
	Endereco  string `bson:"endereco"`
	Cidade    string `bson:"cidade"`
	Descricao string `bson:"descricao"`
	Valor     string `bson:"valor"`
}

type MongoRepository struct {
	client     *mongo.Client
	collection *mongo.Collection
}

func NewMongoRepository(uri, dbName, collectionName string) (*MongoRepository, error) {
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}

	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, err
	}

	collection := client.Database(dbName).Collection(collectionName)
	return &MongoRepository{client: client, collection: collection}, nil
}

func (r *MongoRepository) Save(ctx context.Context, property Property) error {
	_, err := r.collection.InsertOne(ctx, property)
	if err != nil {
		return fmt.Errorf("failed to insert property: %v", err)
	}
	return nil
}

func (r *MongoRepository) FindAll(ctx context.Context) ([]Property, error) {
	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to find properties: %v", err)
	}
	defer cursor.Close(ctx)

	var properties []Property
	if err := cursor.All(ctx, &properties); err != nil {
		return nil, fmt.Errorf("failed to decode properties: %v", err)
	}

	return properties, nil
}

func (r *MongoRepository) Close() {
	if err := r.client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}
