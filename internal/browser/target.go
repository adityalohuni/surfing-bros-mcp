package browser

import "context"

type Target struct {
	SessionID string
	TabID     int
}

type targetKey struct{}

func WithTarget(ctx context.Context, target Target) context.Context {
	if target.SessionID == "" && target.TabID == 0 {
		return ctx
	}
	return context.WithValue(ctx, targetKey{}, target)
}

func TargetFromContext(ctx context.Context) (Target, bool) {
	val := ctx.Value(targetKey{})
	if val == nil {
		return Target{}, false
	}
	target, ok := val.(Target)
	return target, ok
}
