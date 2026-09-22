package userprofile

import (
	"context"

	"example.com/testapp/internal/shared/validator"
)

// ValidationError reports that a request failed validation.
//
// The handler turns it into a 422 so callers get a field-level explanation
// instead of a generic failure.
type ValidationError struct {
	// Errors lists every field that failed validation.
	Errors validator.Errors
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return "validation failed: " + e.Errors.Error()
}

// Service implements the business rules of the user-profile feature.
type Service struct {
	repo Repository
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Create validates req and stores a new UserProfile.
func (s *Service) Create(ctx context.Context, req CreateUserProfileRequest) (UserProfile, error) {
	check := validator.New()
	check.Required("name", req.Name)
	if err := check.Err(); err != nil {
		return UserProfile{}, &ValidationError{Errors: err.(validator.Errors)}
	}

	return s.repo.Create(ctx, UserProfile{Name: req.Name})
}

// List returns every stored UserProfile.
func (s *Service) List(ctx context.Context) ([]UserProfile, error) {
	return s.repo.List(ctx)
}
