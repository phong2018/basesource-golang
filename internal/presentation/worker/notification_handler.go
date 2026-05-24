package worker

import (
	"encoding/json"
	"log/slog"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
)

func HandleNotificationTask(body []byte) error {
	var n domainModel.Notification
	if err := json.Unmarshal(body, &n); err != nil {
		return err
	}
	slog.Info("notification task received", "to", n.To, "subject", n.Subject)
	return nil
}
