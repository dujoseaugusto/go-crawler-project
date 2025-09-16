# Sistema de Sites por Cidade - Documentação

## 📋 Visão Geral

O sistema de sites por cidade foi implementado para permitir descoberta automática e gerenciamento dinâmico de sites de imobiliárias organizados por cidade, substituindo a dependência estática do arquivo `sites.json`.

## 🏗️ Arquitetura

### Componentes Principais

1. **Modelo de Dados** (`CitySites`)
   - Armazena informações de cidades e seus sites
   - Inclui estatísticas e metadados de performance
   - Suporte a validação e limpeza automática

2. **Repositório MongoDB** (`CitySitesRepository`)
   - Interface para operações CRUD
   - Agregações e consultas otimizadas
   - Índices para performance

3. **Engine de Descoberta** (`SiteDiscoveryEngine`)
   - Múltiplas estratégias de busca
   - Processamento assíncrono
   - Validação automática de sites

4. **Serviço de Gerenciamento** (`CitySitesService`)
   - Coordena todas as operações
   - Integração com sistema de crawling existente
   - APIs de alto nível

5. **Handlers da API** (`CitySitesHandler`)
   - Endpoints RESTful
   - Validação de entrada
   - Respostas padronizadas

## 🔍 Estratégias de Descoberta

### 1. Google Search Strategy
- Busca por termos específicos de imobiliárias
- Extrai resultados orgânicos
- Rate limiting para evitar bloqueios

### 2. Directory Search Strategy
- Consulta diretórios conhecidos (Viva Real, ZAP, OLX)
- Extrai links externos de imobiliárias
- Filtragem por relevância

### 3. Domain Guessing Strategy
- Tenta padrões comuns de domínios
- Validação de conteúdo
- Fallback para casos específicos

## 🌐 Endpoints da API

### Descoberta de Sites
```http
POST /cities/discover-sites
Content-Type: application/json

{
  "cities": [
    {"name": "Muzambinho", "state": "MG"},
    {"name": "Alfenas", "state": "MG"}
  ],
  "options": {
    "max_sites_per_city": 20,
    "enable_validation": true
  }
}
```

### Gerenciamento de Cidades
```http
GET /cities                          # Lista todas as cidades
GET /cities/{city}?state=MG          # Busca cidade específica
GET /cities/{city}/sites             # Sites de uma cidade
POST /cities/{city}/validate         # Valida sites da cidade
DELETE /cities/{city}?confirm=true   # Remove cidade
```

### Gerenciamento de Sites
```http
POST /cities/{city}/sites            # Adiciona site manualmente
DELETE /cities/{city}/sites/{url}    # Remove site
PUT /cities/{city}/sites/{url}/stats # Atualiza estatísticas
```

### Jobs de Descoberta
```http
GET /cities/discovery/jobs           # Jobs ativos
GET /cities/discovery/jobs/{job_id}  # Status de job específico
```

### Estatísticas e Limpeza
```http
GET /cities/statistics               # Estatísticas gerais
POST /cities/cleanup                 # Limpeza de sites inativos
GET /cities/region/{region}          # Cidades por região
```

## 🔄 Integração com Sistema Existente

### Crawler Trigger Atualizado
```http
POST /crawler/trigger
Content-Type: application/json

{
  "cities": ["Muzambinho", "Alfenas"],  # Opcional
  "mode": "incremental"
}
```

- **Sem `cities`**: Usa todos os sites ativos do banco
- **Com `cities`**: Usa apenas sites das cidades especificadas
- **Fallback**: Se banco não disponível, usa `sites.json`

### PropertyService Modificado
- Novo construtor: `NewPropertyServiceWithCitySites()`
- Método `ForceCrawling()` aceita parâmetro `cities []string`
- Carregamento dinâmico de URLs do banco
- Fallback automático para `sites.json`

## 📊 Estrutura de Dados

### CitySites
```go
type CitySites struct {
    ID            string     `bson:"_id,omitempty"`
    City          string     `bson:"city"`
    State         string     `bson:"state"`
    Region        string     `bson:"region,omitempty"`
    Sites         []SiteInfo `bson:"sites"`
    LastUpdated   time.Time  `bson:"last_updated"`
    LastDiscovery time.Time  `bson:"last_discovery"`
    Status        string     `bson:"status"`
    TotalSites    int        `bson:"total_sites"`
    ActiveSites   int        `bson:"active_sites"`
    Hash          string     `bson:"hash"`
}
```

### SiteInfo
```go
type SiteInfo struct {
    URL             string    `bson:"url"`
    Name            string    `bson:"name,omitempty"`
    Domain          string    `bson:"domain"`
    Status          string    `bson:"status"`
    DiscoveredAt    time.Time `bson:"discovered_at"`
    LastCrawled     time.Time `bson:"last_crawled,omitempty"`
    PropertiesFound int       `bson:"properties_found"`
    ErrorCount      int       `bson:"error_count"`
    SuccessRate     float64   `bson:"success_rate"`
    DiscoveryMethod string    `bson:"discovery_method,omitempty"`
    ResponseTime    float64   `bson:"response_time,omitempty"`
    LastError       string    `bson:"last_error,omitempty"`
}
```

## 🚀 Como Usar

### 1. Descobrir Sites para Cidades
```bash
curl -X POST http://localhost:8080/cities/discover-sites \
  -H "Content-Type: application/json" \
  -d '{
    "cities": [
      {"name": "Muzambinho", "state": "MG"},
      {"name": "Alfenas", "state": "MG"}
    ],
    "options": {
      "max_sites_per_city": 15,
      "enable_validation": true
    }
  }'
```

### 2. Verificar Status do Job
```bash
curl http://localhost:8080/cities/discovery/jobs/{job_id}
```

### 3. Executar Crawling por Cidades
```bash
curl -X POST http://localhost:8080/crawler/trigger \
  -H "Content-Type: application/json" \
  -d '{
    "cities": ["Muzambinho", "Alfenas"]
  }'
```

### 4. Ver Sites Descobertos
```bash
curl http://localhost:8080/cities/Muzambinho/sites?state=MG
```

## 🔧 Configuração

### Variáveis de Ambiente
```env
MONGO_URI=mongodb://localhost:27017
MONGO_DB=crawler
```

### Inicialização no main.go
```go
// Repositório de sites por cidade
citySitesRepo, err := repository.NewMongoCitySitesRepository(cfg.MongoURI, "crawler")
if err != nil {
    log.Printf("Warning: City sites repository not available: %v", err)
}

// Serviços
propertyService := service.NewPropertyServiceWithCitySites(repo, urlRepo, citySitesRepo, cfg)
citySitesService := service.NewCitySitesService(citySitesRepo)

// Router com suporte a city sites
router := api.SetupRouterWithCitySites(propertyService, citySitesService)
```

## 📈 Benefícios

1. **Descoberta Automática**: Não precisa mais adicionar sites manualmente
2. **Escalabilidade**: Suporte a múltiplas cidades simultaneamente
3. **Flexibilidade**: Crawling por cidades específicas ou todas
4. **Monitoramento**: Estatísticas e performance de cada site
5. **Manutenção**: Limpeza automática de sites inativos
6. **Compatibilidade**: Fallback para `sites.json` se necessário

## 🧪 Testes

### Executar Testes
```bash
# Testes do modelo
go test ./internal/repository/city_sites_test.go ./internal/repository/city_sites.go -v

# Testes do serviço
go test ./internal/service/city_sites_service_test.go ./internal/service/city_sites_service.go -v
```

### Cobertura de Testes
- ✅ Modelo de dados (CitySites)
- ✅ Operações CRUD básicas
- ✅ Serviço de gerenciamento
- ✅ Validações e edge cases

## 🔮 Próximos Passos

1. **Interface Web**: Adicionar UI para gerenciar cidades e sites
2. **Métricas Avançadas**: Dashboard com analytics de performance
3. **Notificações**: Alertas quando sites ficam inativos
4. **Cache**: Redis para otimizar consultas frequentes
5. **Backup**: Sistema de backup automático dos dados
6. **API Rate Limiting**: Controle mais granular por endpoint

## 📝 Logs e Monitoramento

O sistema gera logs estruturados para:
- Descoberta de sites
- Jobs de processamento
- Erros e falhas
- Performance de sites
- Operações de limpeza

Exemplo de log:
```json
{
  "timestamp": "2025-09-15T18:07:14Z",
  "level": "INFO",
  "message": "Sites discovered for city",
  "component": "site_discovery",
  "fields": {
    "city": "Muzambinho",
    "state": "MG",
    "sites": 12,
    "duration": "45.2s"
  }
}
```

---

**Versão**: 1.0  
**Data**: Setembro 2025  
**Autor**: Sistema Go Crawler

