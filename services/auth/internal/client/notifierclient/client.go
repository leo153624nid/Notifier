package notifierclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	notificationv1 "contracts/gen/notifications/v1"
)

type Client struct {
	api  notificationv1.NotificationServiceClient
	conn *grpc.ClientConn
}

func Dial(addr string) (*Client, error) {
	const op = "notifierclient.Dial"

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Client{
		api:  notificationv1.NewNotificationServiceClient(conn),
		conn: conn,
	}, nil
}

func (c *Client) Notify(ctx context.Context, email string) error {
	req := notificationv1.CreateNotificationRequest{
		Recipient: email,
		Subject:   "Authorization",
		Body:      "aprove authorization",
		Channel:   "email",
		Urgent:    true,
	}

	_, err := c.api.CreateNotification(ctx, &req)

	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}
