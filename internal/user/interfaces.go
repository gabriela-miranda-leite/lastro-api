package user

import "context"

type repository interface {
	findByID(ctx context.Context, userID string) (MeResponse, error)
	update(ctx context.Context, userID string, input UpdateInput) error
	findPasswordHash(ctx context.Context, userID string) (string, error)
	softDelete(ctx context.Context, userID, passwordHash string) error
	buildExport(ctx context.Context, userID string) (ExportData, error)
	insertAuditLog(ctx context.Context, userID, action, ip string) error
}
