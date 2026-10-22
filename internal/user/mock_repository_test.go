package user

import "context"

type mockRepository struct {
	findByIDFn           func(ctx context.Context, userID string) (MeResponse, error)
	updateFn             func(ctx context.Context, userID string, input UpdateInput) error
	findPasswordHashFn   func(ctx context.Context, userID string) (string, error)
	updatePasswordHashFn func(ctx context.Context, userID, newHash string) error
	softDeleteFn         func(ctx context.Context, userID, passwordHash string) error
	buildExportFn        func(ctx context.Context, userID string) (ExportData, error)
	insertAuditLogFn     func(ctx context.Context, userID, action, ip string) error
}

func (m *mockRepository) findByID(ctx context.Context, userID string) (MeResponse, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID)
	}
	return MeResponse{ID: userID}, nil
}

func (m *mockRepository) update(ctx context.Context, userID string, input UpdateInput) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, userID, input)
	}
	return nil
}

func (m *mockRepository) findPasswordHash(ctx context.Context, userID string) (string, error) {
	if m.findPasswordHashFn != nil {
		return m.findPasswordHashFn(ctx, userID)
	}
	return "", nil
}

func (m *mockRepository) updatePasswordHash(ctx context.Context, userID, newHash string) error {
	if m.updatePasswordHashFn != nil {
		return m.updatePasswordHashFn(ctx, userID, newHash)
	}
	return nil
}

func (m *mockRepository) softDelete(ctx context.Context, userID, passwordHash string) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, userID, passwordHash)
	}
	return nil
}

func (m *mockRepository) buildExport(ctx context.Context, userID string) (ExportData, error) {
	if m.buildExportFn != nil {
		return m.buildExportFn(ctx, userID)
	}
	return ExportData{}, nil
}

func (m *mockRepository) insertAuditLog(ctx context.Context, userID, action, ip string) error {
	if m.insertAuditLogFn != nil {
		return m.insertAuditLogFn(ctx, userID, action, ip)
	}
	return nil
}
