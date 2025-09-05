package service

import (
	"context"
	"errors"

	"go-crawler-project/internal/repository"
)

type PropertyService struct {
	repo repository.PropertyRepository
}

func NewPropertyService(repo repository.PropertyRepository) *PropertyService {
	return &PropertyService{repo: repo}
}

func (s *PropertyService) SaveProperty(ctx context.Context, property repository.Property) error {
	if property.Address == "" || property.City == "" || property.Description == "" || property.Value == 0 {
		return errors.New("property fields cannot be empty")
	}
	return s.repo.Save(ctx, property)
}

func (s *PropertyService) GetAllProperties(ctx context.Context) ([]repository.Property, error) {
	return s.repo.FindAll(ctx)
}