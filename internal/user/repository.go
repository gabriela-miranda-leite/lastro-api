package user

// Repository handles data access for the user domain.
type Repository struct{}

// NewRepository creates a new Repository.
func NewRepository() *Repository {
	return &Repository{}
}
