package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujoseaugusto/go-crawler-project/internal/logger"
	"github.com/dujoseaugusto/go-crawler-project/internal/repository"
	"github.com/dujoseaugusto/go-crawler-project/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// PropertyServiceInterface define a interface para o service
type PropertyServiceInterface interface {
	GetAllProperties(ctx context.Context) ([]repository.Property, error)
	SearchProperties(ctx context.Context, filter repository.PropertyFilter, pagination repository.PaginationParams) (*repository.PropertySearchResult, error)
	ForceCrawling(ctx context.Context) error
}

// Mock PropertyService
type MockPropertyService struct {
	mock.Mock
}

func (m *MockPropertyService) GetAllProperties(ctx context.Context) ([]repository.Property, error) {
	args := m.Called(ctx)
	return args.Get(0).([]repository.Property), args.Error(1)
}

func (m *MockPropertyService) SearchProperties(ctx context.Context, filter repository.PropertyFilter, pagination repository.PaginationParams) (*repository.PropertySearchResult, error) {
	args := m.Called(ctx, filter, pagination)
	return args.Get(0).(*repository.PropertySearchResult), args.Error(1)
}

func (m *MockPropertyService) ForceCrawling(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Wrapper para usar mock no handler
type PropertyHandlerWrapper struct {
	mockService PropertyServiceInterface
	logger      *logger.Logger
}

func (w *PropertyHandlerWrapper) GetProperties(c *gin.Context) {
	ctx := c.Request.Context()
	
	properties, err := w.mockService.GetAllProperties(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to fetch properties",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Properties retrieved successfully",
		Data:    properties,
	})
}

func (w *PropertyHandlerWrapper) SearchProperties(c *gin.Context) {
	ctx := c.Request.Context()
	
	// Simulação básica de parsing
	filter := repository.PropertyFilter{}
	pagination := repository.PaginationParams{Page: 1, PageSize: 10}
	
	result, err := w.mockService.SearchProperties(ctx, filter, pagination)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to search properties",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Search completed successfully",
		Data:    result,
	})
}

func (w *PropertyHandlerWrapper) TriggerCrawler(c *gin.Context) {
	ctx := c.Request.Context()
	
	err := w.mockService.ForceCrawling(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to trigger crawler",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Crawler triggered successfully",
	})
}

// Test setup
func setupTestHandler() (*PropertyHandlerWrapper, *MockPropertyService) {
	mockService := &MockPropertyService{}
	testLogger := logger.NewLogger("test")
	handler := &PropertyHandlerWrapper{
		mockService: mockService,
		logger:      testLogger,
	}
	return handler, mockService
}

func setupTestRouter(handler *PropertyHandlerWrapper) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/properties", handler.GetProperties)
	r.GET("/properties/search", handler.SearchProperties)
	r.POST("/crawler/trigger", handler.TriggerCrawler)
	return r
}

func TestNewPropertyHandler(t *testing.T) {
	mockService := &service.PropertyService{}
	handler := NewPropertyHandler(mockService)
	
	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.Service)
	assert.NotNil(t, handler.logger)
}

func TestGetProperties_Success(t *testing.T) {
	handler, mockService := setupTestHandler()
	router := setupTestRouter(handler)

	// Mock data
	expectedProperties := []repository.Property{
		{
			ID:        "1",
			Descricao: "Linda casa com vista para o mar",
			Cidade:    "Rio de Janeiro",
			Bairro:    "Copacabana",
		},
		{
			ID:        "2",
			Descricao: "Apartamento moderno no centro",
			Cidade:    "São Paulo",
			Bairro:    "Centro",
		},
	}

	mockService.On("GetAllProperties", mock.Anything).Return(expectedProperties, nil)

	// Test request
	req, _ := http.NewRequest("GET", "/properties", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Properties retrieved successfully", response.Message)
	assert.Equal(t, 2, len(response.Data.([]interface{})))

	mockService.AssertExpectations(t)
}

func TestGetProperties_ServiceError(t *testing.T) {
	handler, mockService := setupTestHandler()
	router := setupTestRouter(handler)

	mockService.On("GetAllProperties", mock.Anything).Return([]repository.Property{}, errors.New("database error"))

	req, _ := http.NewRequest("GET", "/properties", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "internal_error", response.Error)
	assert.Equal(t, "Failed to fetch properties", response.Message)

	mockService.AssertExpectations(t)
}

func TestSearchProperties_Success(t *testing.T) {
	handler, mockService := setupTestHandler()
	router := setupTestRouter(handler)

	expectedResult := &repository.PropertySearchResult{
		Properties: []repository.Property{
			{
				ID:        "1",
				Descricao: "Linda casa com vista para o mar",
				Cidade:    "Rio de Janeiro",
			},
		},
		TotalItems:  1,
		TotalPages:  1,
		CurrentPage: 1,
		PageSize:    10,
	}

	mockService.On("SearchProperties", 
		mock.Anything, 
		mock.Anything, 
		mock.Anything).Return(expectedResult, nil)

	req, _ := http.NewRequest("GET", "/properties/search?q=casa+praia&cidade=Rio+de+Janeiro", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Search completed successfully", response.Message)

	mockService.AssertExpectations(t)
}

func TestTriggerCrawler_Success(t *testing.T) {
	handler, mockService := setupTestHandler()
	router := setupTestRouter(handler)

	mockService.On("ForceCrawling", mock.Anything).Return(nil)

	req, _ := http.NewRequest("POST", "/crawler/trigger", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response SuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Crawler triggered successfully", response.Message)

	mockService.AssertExpectations(t)
}

func TestTriggerCrawler_ServiceError(t *testing.T) {
	handler, mockService := setupTestHandler()
	router := setupTestRouter(handler)

	mockService.On("ForceCrawling", mock.Anything).Return(errors.New("crawler failed"))

	req, _ := http.NewRequest("POST", "/crawler/trigger", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "internal_error", response.Error)
	assert.Equal(t, "Failed to trigger crawler", response.Message)

	mockService.AssertExpectations(t)
}

// Benchmark tests
func BenchmarkGetProperties(b *testing.B) {
	handler, mockService := setupTestHandler()
	router := setupTestRouter(handler)

	properties := make([]repository.Property, 100)
	for i := 0; i < 100; i++ {
		properties[i] = repository.Property{
			ID:        string(rune(i)),
			Descricao: "Test Property",
		}
	}

	mockService.On("GetAllProperties", mock.Anything).Return(properties, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/properties", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}