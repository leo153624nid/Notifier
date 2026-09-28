package service

import (
	"context"
	"fmt"
)

func (s *NotificationService) Delete(
	ctx context.Context,
	id int,
) error {
	const op = "NotificationService.Delete"

	err := s.repo.DeleteById(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
