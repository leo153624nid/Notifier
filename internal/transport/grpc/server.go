package grpc

import (
	"context"
	"errors"
	"log/slog"

	notificationv1 "contracts/gen/notifications/v1"
	"notifier/internal/domain"
	"notifier/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Router struct {
	notificationv1.UnimplementedNotificationServiceServer
	notifications *service.NotificationService
	logger        *slog.Logger
}

func NewRouter(notifications *service.NotificationService, logger *slog.Logger) *Router {
	return &Router{
		notifications: notifications,
		logger:        logger,
	}
}

func NewGRPCServer(notifications *service.NotificationService, logger *slog.Logger) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			requestIDInterceptor,
			loggingRecoveryInterceptor(logger),
		),
	)
	notificationv1.RegisterNotificationServiceServer(srv, NewRouter(notifications, logger))
	return srv
}

func (r *Router) CreateNotification(
	ctx context.Context,
	req *notificationv1.CreateNotificationRequest,
) (*notificationv1.CreateNotificationResponse, error) {
	const op = "Server.CreateNotification"

	n := domain.Notification{
		Recipient: req.GetRecipient(),
		Subject:   req.GetSubject(),
		Body:      req.GetBody(),
		Channel:   req.GetChannel(),
		IsUrgent:  req.GetUrgent(),
	}

	requestID := requestIDFromContext(ctx)

	created, err := r.notifications.Create(ctx, n, requestID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidNotification):
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		case errors.Is(err, service.ErrUnsupportedChannel):
			return nil, status.Error(codes.InvalidArgument, "unsupported channel")
		default:
			r.logger.Error("create notification failed", "op", op, "error", err)
			return nil, status.Error(codes.Internal, "internal error")
		}
	}

	return &notificationv1.CreateNotificationResponse{
		Id:     int64(created.ID),
		Status: created.Status,
	}, nil
}
