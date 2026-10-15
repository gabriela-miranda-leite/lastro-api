package auth

// Repository handles data access for the auth domain.
type Repository struct{}

// NewRepository creates a new Repository.
func NewRepository() *Repository {
	return &Repository{}
}
