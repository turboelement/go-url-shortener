package audit

import (
	"context"
	"net/http"
)

type auditSubjectKey struct{}

func WithSubject(ctx context.Context, auditSubject *Subject) context.Context {
	return context.WithValue(ctx, auditSubjectKey{}, auditSubject)
}

func FromContext(ctx context.Context) *Subject {
	if s, ok := ctx.Value(auditSubjectKey{}).(*Subject); ok && s != nil {
		return s
	}
	return nil
}

func Middleware(auditSubject *Subject) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithSubject(r.Context(), auditSubject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
