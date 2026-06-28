// Package grpcserver implements the gRPC server for the URL shortener service.
package grpcserver

import (
	"context"

	"go-url-shortener/internal/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authorizationKey = "authorization"
	userIDKey        = "user_id"
)

// AuthInterceptor returns a UnaryServerInterceptor that:
// - extracts the JWT from the "authorization" metadata,
// - if the token is valid, places the userID into the context,
// - if no token is present, generates a new userID and sends it back in a response header,
// - if the token is invalid, returns an Unauthenticated error.
func AuthInterceptor(cookieSecret string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		token := ""
		if vals := md.Get(authorizationKey); len(vals) > 0 {
			token = vals[0]
		}

		var userID string

		if token == "" {
			// No token — generate a new user
			newID, err := auth.GenerateUserID()
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to generate user ID: %v", err)
			}
			userID = newID

			// Sign a token and send it back in a response header
			signedToken, err := auth.SignUserID(userID, cookieSecret)
			if err == nil {
				_ = grpc.SetHeader(ctx, metadata.Pairs(userIDKey, signedToken))
			}
		} else {
			// Token present — try to verify
			uid, ok := auth.VerifyUserID(token, cookieSecret)
			if !ok {
				return nil, status.Errorf(codes.Unauthenticated, "invalid or expired token")
			}
			userID = uid
		}

		ctx = auth.SetUserIDInContext(ctx, userID)

		return handler(ctx, req)
	}
}

// GetUserIDFromGRPCContext extracts the user ID from a gRPC context.
func GetUserIDFromGRPCContext(ctx context.Context) (string, error) {
	return auth.GetUserIDFromContext(ctx)
}
