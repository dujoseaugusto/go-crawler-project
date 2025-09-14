package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/dujoseaugusto/go-crawler-project/internal/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PropertyFilter define os filtros disponíveis para busca
type PropertyFilter struct {
	Query        string  `json:"q,omitempty"`           // Busca inteligente por palavras-chave
	Cidade       string  `json:"cidade,omitempty"`
	Bairro       string  `json:"bairro,omitempty"`
	TipoImovel   string  `json:"tipo_imovel,omitempty"`
	ValorMin     float64 `json:"valor_min,omitempty"`
	ValorMax     float64 `json:"valor_max,omitempty"`
	QuartosMin   int     `json:"quartos_min,omitempty"`
	QuartosMax   int     `json:"quartos_max,omitempty"`
	BanheirosMin int     `json:"banheiros_min,omitempty"`
	BanheirosMax int     `json:"banheiros_max,omitempty"`
	AreaMin      float64 `json:"area_min,omitempty"`
	AreaMax      float64 `json:"area_max,omitempty"`
}

// PaginationParams define os parâmetros de paginação
type PaginationParams struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

// PropertySearchResult define o resultado da busca com metadados
type PropertySearchResult struct {
	Properties  []Property `json:"properties"`
	TotalItems  int64      `json:"total_items"`
	TotalPages  int        `json:"total_pages"`
	CurrentPage int        `json:"current_page"`
	PageSize    int        `json:"page_size"`
}

// PropertyRepository define as operações que podem ser realizadas no repositório de propriedades
type PropertyRepository interface {
	Save(ctx context.Context, property Property) error
	FindAll(ctx context.Context) ([]Property, error)
	FindWithFilters(ctx context.Context, filter PropertyFilter, pagination PaginationParams) (*PropertySearchResult, error)
	ClearAll(ctx context.Context) error
	Close()
}

type Property struct {
	ID              string   `bson:"_id,omitempty" json:"id"`
	Hash            string   `bson:"hash" json:"hash"`
	Endereco        string   `bson:"endereco" json:"endereco"`
	Cidade          string   `bson:"cidade" json:"cidade"`
	Bairro          string   `bson:"bairro" json:"bairro"`
	CEP             string   `bson:"cep" json:"cep"`
	Descricao       string   `bson:"descricao" json:"descricao"`
	Valor           float64  `bson:"valor" json:"valor"`
	ValorTexto      string   `bson:"valor_texto" json:"valor_texto"`
	Quartos         int      `bson:"quartos" json:"quartos"`
	Banheiros       int      `bson:"banheiros" json:"banheiros"`
	AreaTotal       float64  `bson:"area_total" json:"area_total"`
	AreaUtil        float64  `bson:"area_util" json:"area_util"`
	TipoImovel      string   `bson:"tipo_imovel" json:"tipo_imovel"`
	URL             string   `bson:"url" json:"url"`
	Caracteristicas []string `bson:"caracteristicas" json:"caracteristicas"`
}

type MongoRepository struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// GeneratePropertyHash gera um hash único baseado nas informações principais do imóvel
func GeneratePropertyHash(property Property) string {
	// Normaliza os dados para gerar hash consistente
	endereco := strings.TrimSpace(strings.ToLower(property.Endereco))
	url := strings.TrimSpace(strings.ToLower(property.URL))
	valor := strconv.FormatFloat(property.Valor, 'f', 2, 64)

	// Combina as informações principais
	data := fmt.Sprintf("%s|%s|%s", endereco, url, valor)

	// Gera hash SHA-256
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
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

	// Cria índice único no campo hash para garantir unicidade
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "hash", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	_, err = collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		log.Printf("Warning: Failed to create unique index on hash field: %v", err)
	} else {
		log.Printf("Unique index created on hash field")
	}

	return &MongoRepository{client: client, collection: collection}, nil
}

func (r *MongoRepository) Save(ctx context.Context, property Property) error {
	// Gera hash único para o imóvel
	property.Hash = GeneratePropertyHash(property)

	// Usa upsert para evitar duplicatas baseado no hash
	filter := bson.M{"hash": property.Hash}
	update := bson.M{"$set": property}

	opts := options.Update().SetUpsert(true)
	result, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save property: %v", err)
	}

	if result.UpsertedCount > 0 {
		log.Printf("Novo imóvel salvo com hash: %s", property.Hash)
	} else if result.ModifiedCount > 0 {
		log.Printf("Imóvel atualizado com hash: %s", property.Hash)
	} else {
		log.Printf("Imóvel já existe com hash: %s", property.Hash)
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

func (r *MongoRepository) FindWithFilters(ctx context.Context, filter PropertyFilter, pagination PaginationParams) (*PropertySearchResult, error) {
	// Construir o filtro MongoDB
	mongoFilter := bson.M{}

	// Busca inteligente por palavras-chave (filtro 'q')
	if filter.Query != "" {
		searchTerms := utils.CreateSearchTerms(filter.Query)
		if len(searchTerms) > 0 {
			// Criar regex pattern para busca flexível
			regexPattern := utils.BuildSearchRegex(searchTerms)
			
			// Buscar em múltiplos campos com OR
			queryConditions := []bson.M{
				{"descricao": bson.M{"$regex": regexPattern, "$options": "i"}},
				{"endereco": bson.M{"$regex": regexPattern, "$options": "i"}},
				{"cidade": bson.M{"$regex": regexPattern, "$options": "i"}},
				{"bairro": bson.M{"$regex": regexPattern, "$options": "i"}},
				{"tipo_imovel": bson.M{"$regex": regexPattern, "$options": "i"}},
				{"caracteristicas": bson.M{"$regex": regexPattern, "$options": "i"}},
			}
			
			mongoFilter["$or"] = queryConditions
		}
	}

	// Filtros específicos com busca case-insensitive e sem acentos
	if filter.Cidade != "" {
		normalizedCidade := utils.NormalizeText(filter.Cidade)
		mongoFilter["cidade"] = bson.M{"$regex": normalizedCidade, "$options": "i"}
	}

	if filter.Bairro != "" {
		normalizedBairro := utils.NormalizeText(filter.Bairro)
		mongoFilter["bairro"] = bson.M{"$regex": normalizedBairro, "$options": "i"}
	}

	if filter.TipoImovel != "" {
		normalizedTipo := utils.NormalizeText(filter.TipoImovel)
		mongoFilter["tipo_imovel"] = bson.M{"$regex": normalizedTipo, "$options": "i"}
	}

	// Filtros de valor
	if filter.ValorMin > 0 || filter.ValorMax > 0 {
		valorFilter := bson.M{}
		if filter.ValorMin > 0 {
			valorFilter["$gte"] = filter.ValorMin
		}
		if filter.ValorMax > 0 {
			valorFilter["$lte"] = filter.ValorMax
		}
		mongoFilter["valor"] = valorFilter
	}

	// Filtros de quartos
	if filter.QuartosMin > 0 || filter.QuartosMax > 0 {
		quartosFilter := bson.M{}
		if filter.QuartosMin > 0 {
			quartosFilter["$gte"] = filter.QuartosMin
		}
		if filter.QuartosMax > 0 {
			quartosFilter["$lte"] = filter.QuartosMax
		}
		mongoFilter["quartos"] = quartosFilter
	}

	// Filtros de banheiros
	if filter.BanheirosMin > 0 || filter.BanheirosMax > 0 {
		banheirosFilter := bson.M{}
		if filter.BanheirosMin > 0 {
			banheirosFilter["$gte"] = filter.BanheirosMin
		}
		if filter.BanheirosMax > 0 {
			banheirosFilter["$lte"] = filter.BanheirosMax
		}
		mongoFilter["banheiros"] = banheirosFilter
	}

	// Filtros de área (usando area_total)
	if filter.AreaMin > 0 || filter.AreaMax > 0 {
		areaFilter := bson.M{}
		if filter.AreaMin > 0 {
			areaFilter["$gte"] = filter.AreaMin
		}
		if filter.AreaMax > 0 {
			areaFilter["$lte"] = filter.AreaMax
		}
		mongoFilter["area_total"] = areaFilter
	}

	// Contar total de documentos
	totalItems, err := r.collection.CountDocuments(ctx, mongoFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to count properties: %v", err)
	}

	// Calcular paginação
	if pagination.PageSize <= 0 {
		pagination.PageSize = 10 // Padrão
	}
	if pagination.Page <= 0 {
		pagination.Page = 1 // Padrão
	}

	totalPages := int((totalItems + int64(pagination.PageSize) - 1) / int64(pagination.PageSize))
	skip := (pagination.Page - 1) * pagination.PageSize

	// Configurar opções de busca
	findOptions := options.Find()
	findOptions.SetSkip(int64(skip))
	findOptions.SetLimit(int64(pagination.PageSize))
	
	// Ordenação inteligente: se há busca por query, ordenar por relevância, senão por valor
	if filter.Query != "" {
		// Para busca por query, ordenar por valor decrescente (pode ser melhorado com score de relevância)
		findOptions.SetSort(bson.D{{Key: "valor", Value: -1}})
	} else {
		// Para filtros normais, ordenar por valor decrescente
		findOptions.SetSort(bson.D{{Key: "valor", Value: -1}})
	}

	// Executar busca
	cursor, err := r.collection.Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to find properties: %v", err)
	}
	defer cursor.Close(ctx)

	var properties []Property
	if err := cursor.All(ctx, &properties); err != nil {
		return nil, fmt.Errorf("failed to decode properties: %v", err)
	}

	// Se há busca por query, calcular e ordenar por relevância
	if filter.Query != "" && len(properties) > 0 {
		properties = r.sortByRelevance(properties, filter.Query)
	}

	return &PropertySearchResult{
		Properties:  properties,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		CurrentPage: pagination.Page,
		PageSize:    pagination.PageSize,
	}, nil
}

// sortByRelevance ordena propriedades por relevância da busca
func (r *MongoRepository) sortByRelevance(properties []Property, query string) []Property {
	searchTerms := utils.CreateSearchTerms(query)
	if len(searchTerms) == 0 {
		return properties
	}

	// Calcular score de relevância para cada propriedade
	type PropertyWithScore struct {
		Property Property
		Score    float64
	}

	var scored []PropertyWithScore
	for _, prop := range properties {
		// Calcular score baseado em múltiplos campos
		descScore := utils.CalculateRelevanceScore(prop.Descricao, searchTerms) * 3.0    // Peso maior para descrição
		enderecoScore := utils.CalculateRelevanceScore(prop.Endereco, searchTerms) * 2.0 // Peso médio para endereço
		cidadeScore := utils.CalculateRelevanceScore(prop.Cidade, searchTerms) * 1.5
		bairroScore := utils.CalculateRelevanceScore(prop.Bairro, searchTerms) * 1.5
		tipoScore := utils.CalculateRelevanceScore(prop.TipoImovel, searchTerms) * 1.0

		totalScore := descScore + enderecoScore + cidadeScore + bairroScore + tipoScore

		scored = append(scored, PropertyWithScore{
			Property: prop,
			Score:    totalScore,
		})
	}

	// Ordenar por score decrescente
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].Score < scored[j].Score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Extrair propriedades ordenadas
	var result []Property
	for _, item := range scored {
		result = append(result, item.Property)
	}

	return result
}

func (r *MongoRepository) ClearAll(ctx context.Context) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to clear all properties: %v", err)
	}
	log.Printf("Banco de dados limpo - todas as propriedades foram removidas")
	return nil
}

func (r *MongoRepository) Close() {
	if err := r.client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}
