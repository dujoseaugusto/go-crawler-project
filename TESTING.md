# Testing Guide

Este documento descreve a estratégia de testes implementada no projeto Go Crawler.

## 📋 Visão Geral

O projeto implementa uma suíte completa de testes incluindo:
- **Testes Unitários**: Para lógica de negócio e funções puras
- **Testes de Integração**: Para interação com MongoDB
- **Mocks**: Para dependências externas
- **Benchmarks**: Para análise de performance
- **Coverage**: Para análise de cobertura de código

## 🏗️ Estrutura de Testes

```
├── api/handler/
│   └── property_handler_test.go     # Testes dos handlers HTTP
├── internal/
│   ├── crawler/
│   │   └── engine_test.go           # Testes do engine de crawling
│   ├── mocks/
│   │   ├── interfaces.go            # Interfaces para mocks
│   │   └── mocks.go                 # Implementações de mocks
│   ├── repository/
│   │   └── mongo_repository_test.go # Testes de integração MongoDB
│   ├── service/
│   │   └── property_service_test.go # Testes dos services
│   └── utils/
│       └── text_utils_test.go       # Testes das funções utilitárias
```

## 🧪 Tipos de Testes

### 1. Testes Unitários

**Localização**: `internal/utils/`, `api/handler/`, `internal/service/`, `internal/crawler/`

**Características**:
- Testam funções isoladas
- Usam mocks para dependências
- Execução rápida
- Não dependem de recursos externos

**Exemplo**:
```go
func TestNormalizeText(t *testing.T) {
    result := NormalizeText("São Paulo")
    assert.Equal(t, "sao paulo", result)
}
```

### 2. Testes de Integração

**Localização**: `internal/repository/`

**Características**:
- Testam interação com MongoDB
- Requerem instância do MongoDB rodando
- Testam fluxos completos de dados
- Usam banco de dados de teste

**Exemplo**:
```go
func (suite *MongoRepositoryTestSuite) TestSaveProperty() {
    property := Property{ID: "test-1", Titulo: "Casa Teste"}
    err := suite.repository.Save(property)
    assert.NoError(suite.T(), err)
}
```

### 3. Mocks

**Localização**: `internal/mocks/`

**Características**:
- Implementam interfaces usando testify/mock
- Permitem controlar comportamento de dependências
- Facilitam testes de cenários de erro
- Verificam chamadas de métodos

**Exemplo**:
```go
type MockPropertyService struct {
    mock.Mock
}

func (m *MockPropertyService) GetAllProperties(filter PropertyFilter) ([]Property, error) {
    args := m.Called(filter)
    return args.Get(0).([]Property), args.Error(1)
}
```

### 4. Benchmarks

**Características**:
- Medem performance de funções críticas
- Identificam gargalos de performance
- Monitoram regressões de performance

**Exemplo**:
```go
func BenchmarkNormalizeText(b *testing.B) {
    text := "São Paulo - Coração de Jesus"
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        NormalizeText(text)
    }
}
```

## 🚀 Executando Testes

### Comandos Básicos

```bash
# Todos os testes
make test

# Apenas testes unitários
make test-unit

# Apenas testes de integração (requer MongoDB)
make test-integration

# Testes com coverage
make test-coverage

# Testes com race detection
make test-verbose

# Benchmarks
make test-bench
```

### Comandos Detalhados

```bash
# Testes específicos por pacote
go test ./internal/utils/... -v
go test ./api/handler/... -v
go test ./internal/service/... -v

# Testes com coverage por função
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

# Testes com HTML coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 🐳 Testes com Docker

### Preparação do Ambiente

```bash
# Iniciar MongoDB para testes de integração
make mongo-start

# Parar MongoDB
make mongo-stop

# Executar testes em container
docker run --rm -v $(pwd):/app -w /app golang:1.24 go test ./...
```

## 📊 Coverage Reports

### Visualização de Coverage

1. **Terminal**:
```bash
make test-coverage-func
```

2. **HTML Report**:
```bash
make test-coverage
# Abre coverage.html no navegador
```

### Metas de Coverage

- **Target Geral**: 80%+
- **Handlers**: 90%+
- **Services**: 85%+
- **Utils**: 95%+
- **Repository**: 70%+ (devido a testes de integração)

## 🔧 Configuração de CI/CD

### GitHub Actions

O projeto inclui pipeline de CI/CD em `.github/workflows/test.yml`:

- **Testes Unitários**: Executados em paralelo
- **Testes de Integração**: Com MongoDB service
- **Linting**: golangci-lint
- **Security**: gosec scanner
- **Build**: Verificação de build
- **Coverage**: Upload para Codecov

### Configuração Local

```bash
# Instalar golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Executar linter
make lint

# Formatar código
make fmt

# Verificar código
make vet
```

## 🎯 Melhores Práticas

### 1. Estrutura de Testes

```go
func TestFunctionName(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"
    
    // Act
    result := FunctionName(input)
    
    // Assert
    assert.Equal(t, expected, result)
}
```

### 2. Table-Driven Tests

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected bool
    }{
        {"valid input", "valid", true},
        {"invalid input", "", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Validate(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 3. Mocks com Verificação

```go
func TestServiceMethod(t *testing.T) {
    mockRepo := &MockRepository{}
    service := NewService(mockRepo)
    
    // Setup mock expectations
    mockRepo.On("Save", mock.AnythingOfType("Property")).Return(nil)
    
    // Execute
    err := service.SaveProperty(property)
    
    // Verify
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

### 4. Testes de Integração com Setup/Teardown

```go
func (suite *TestSuite) SetupTest() {
    // Clean database before each test
    suite.repository.DeleteAll()
}

func (suite *TestSuite) TearDownSuite() {
    // Close connections after all tests
    suite.repository.Close()
}
```

## 🚨 Troubleshooting

### Problemas Comuns

1. **MongoDB não disponível**:
```bash
# Verificar se MongoDB está rodando
docker ps | grep mongo

# Iniciar MongoDB se necessário
make mongo-start
```

2. **Testes falhando por timeout**:
```bash
# Aumentar timeout
go test ./... -timeout 30s
```

3. **Coverage baixo**:
```bash
# Identificar arquivos não cobertos
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep -v "100.0%"
```

4. **Race conditions**:
```bash
# Executar com race detector
go test ./... -race
```

## 📈 Métricas de Qualidade

### Métricas Atuais (Estimadas)

- **Total de Testes**: 50+ casos de teste
- **Coverage**: 85%+
- **Benchmarks**: 10+ funções críticas
- **Mocks**: Todas as dependências externas
- **CI/CD**: Pipeline completo

### Monitoramento Contínuo

- **GitHub Actions**: Execução automática em PRs
- **Codecov**: Tracking de coverage
- **golangci-lint**: Qualidade de código
- **gosec**: Análise de segurança

## 🔄 Próximos Passos

1. **Testes E2E**: Implementar testes end-to-end
2. **Performance Tests**: Testes de carga e stress
3. **Mutation Testing**: Verificar qualidade dos testes
4. **Property-Based Testing**: Testes com dados gerados
5. **Contract Testing**: Testes de API contracts
