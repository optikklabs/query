package contracts

import "context"

type TenantContext struct {
	TeamID    int64
	UserID    int64
	UserEmail string
	UserRole  string
}

type tenantCtxKey struct{}

// WithTenant returns a context carrying the authenticated tenant.
func WithTenant(ctx context.Context, t TenantContext) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, t)
}

func TenantFrom(ctx context.Context) TenantContext {
	t, _ := ctx.Value(tenantCtxKey{}).(TenantContext)
	return t
}
