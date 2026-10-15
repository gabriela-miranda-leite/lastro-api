package transaction

// Repository handles data access for the transaction domain.
type Repository struct{}

// NewRepository creates a new Repository.
func NewRepository() *Repository {
	return &Repository{}
}
