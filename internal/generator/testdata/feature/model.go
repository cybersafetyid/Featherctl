package userprofile

// UserProfile is the domain model for the user-profile feature.
type UserProfile struct {
	// ID is assigned by the repository when the user-profile is stored.
	ID string `json:"id"`
	// Name is the human readable label of the user-profile.
	Name string `json:"name"`
}

// CreateUserProfileRequest is the JSON payload accepted by the create
// endpoint.
type CreateUserProfileRequest struct {
	Name string `json:"name"`
}

// UserProfileResponse is the JSON representation of a UserProfile.
type UserProfileResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ToResponse maps the domain model onto its HTTP representation.
//
// Keeping the mapping in one place means the wire format can change without
// touching the handler or the service.
func (m UserProfile) ToResponse() UserProfileResponse {
	return UserProfileResponse{
		ID:   m.ID,
		Name: m.Name,
	}
}
