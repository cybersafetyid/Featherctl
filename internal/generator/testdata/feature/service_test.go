package userprofile

import (
	"context"
	"errors"
	"testing"
)

// stubRepository records what the service asked it to do and returns canned
// values, so service tests never touch the real implementation.
type stubRepository struct {
	created []UserProfile
	listErr error
}

// Create implements [Repository].
func (s *stubRepository) Create(_ context.Context, item UserProfile) (UserProfile, error) {
	item.ID = "stub-id"
	s.created = append(s.created, item)
	return item, nil
}

// List implements [Repository].
func (s *stubRepository) List(_ context.Context) ([]UserProfile, error) {
	return s.created, s.listErr
}

func TestServiceCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request CreateUserProfileRequest
		wantErr bool
	}{
		{name: "accepts a valid request", request: CreateUserProfileRequest{Name: "first"}},
		{name: "rejects a blank name", request: CreateUserProfileRequest{Name: "   "}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &stubRepository{}
			service := NewService(repo)

			got, err := service.Create(context.Background(), test.request)

			if test.wantErr {
				var invalid *ValidationError
				if !errors.As(err, &invalid) {
					t.Fatalf("error = %v, want *ValidationError", err)
				}
				if len(repo.created) != 0 {
					t.Errorf("repository was called %d times for invalid input, want 0", len(repo.created))
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != "stub-id" {
				t.Errorf("id = %q, want %q", got.ID, "stub-id")
			}
			if got.Name != test.request.Name {
				t.Errorf("name = %q, want %q", got.Name, test.request.Name)
			}
		})
	}
}

func TestServiceListPropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("boom")
	service := NewService(&stubRepository{listErr: want})

	_, err := service.List(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
