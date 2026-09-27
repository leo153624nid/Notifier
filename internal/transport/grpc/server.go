package transport_grpc

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notificationv1 "contracts/gen/notifications/v1"
	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
	"notifier/internal/service"
)

type Router struct {
	notificationv1.UnimplementedNotificationServiceServer
	notifications *service.NotificationService
}

func NewRouter(
	notifications *service.NotificationService,
) *Router {
	return &Router{
		notifications: notifications,
	}
}

func NewGRPCServer(
	notifications *service.NotificationService,
	logger *core_logger.Logger,
) *grpc.Server {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			requestIDInterceptor,
			loggingRecoveryInterceptor(logger),
		),
	)
	notificationv1.RegisterNotificationServiceServer(
		srv,
		NewRouter(notifications),
	)

	return srv
}

func (r *Router) CreateNotification(
	ctx context.Context,
	req *notificationv1.CreateNotificationRequest,
) (*notificationv1.CreateNotificationResponse, error) {
	const op = "Server.CreateNotification"

	logger := core_logger.FromContext(ctx)

	n := domain.Notification{
		Recipient: req.GetRecipient(),
		Subject:   req.GetSubject(),
		Body:      req.GetBody(),
		Channel:   req.GetChannel(),
		IsUrgent:  req.GetUrgent(),
	}

	created, err := r.notifications.Create(ctx, n)
	if err != nil {
		switch {
		case errors.Is(err, core_errors.ErrInvalidNotification):
			logger.Warn(
				"invalid request",
				zap.String("op", op),
				zap.Error(core_errors.ErrInvalidNotification),
			)
			return nil, status.Error(codes.InvalidArgument, "invalid request")
		case errors.Is(err, core_errors.ErrUnsupportedChannel):
			logger.Warn(
				"unsupported channel",
				zap.String("op", op),
				zap.Error(core_errors.ErrUnsupportedChannel),
			)
			return nil, status.Error(codes.InvalidArgument, "unsupported channel")
		default:
			logger.Error(
				"create notification failed",
				zap.String("op", op),
				zap.Error(err),
			)
			return nil, status.Error(codes.Internal, "internal error")
		}
	}

	return &notificationv1.CreateNotificationResponse{
		Id:     int64(created.ID),
		Status: created.Status,
	}, nil
}
