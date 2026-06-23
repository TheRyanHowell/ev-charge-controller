package workers

import (
	"context"
	"testing"
)

func TestCheckPendingSessionTimeout_Success(t *testing.T) {
	service, db := setupTestService(t)
	defer db.Close()
	defer service.Shutdown()

	ctx := context.Background()
	checkPendingSessionTimeout(ctx, service)
}

func TestCheckPendingSessionTimeout_ServiceError(t *testing.T) {
	service, db := setupTestService(t)
	defer service.Shutdown()

	db.Close()

	ctx := context.Background()
	checkPendingSessionTimeout(ctx, service)
}
