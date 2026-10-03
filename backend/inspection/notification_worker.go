package inspection

import "context"

type NotificationWorker struct {
	Store  Store
	Sender WebhookSender
}

func (w NotificationWorker) RunOnce(ctx context.Context) (bool, error) {
	d, c, ok, err := w.Store.ClaimDelivery(ctx)
	if err != nil || !ok {
		return ok, err
	}
	f := Finding{}
	if d.FindingID != "" {
		f, err = w.Store.GetFinding(ctx, d.FindingID)
		if err != nil {
			return true, w.Store.FinishDelivery(ctx, d, 0, "", err)
		}
	}
	eventType := d.EventType
	if eventType == "" {
		eventType = "inspection.finding." + f.Status
	}
	status, body, err := w.Sender.Send(ctx, c, nil, WebhookEvent{Type: eventType, Finding: f, RunID: d.RunID, Data: d.EventPayload})
	return true, w.Store.FinishDelivery(ctx, d, status, body, err)
}
