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
	f, err := w.Store.GetFinding(ctx, d.FindingID)
	if err != nil {
		return true, w.Store.FinishDelivery(ctx, d, 0, "", err)
	}
	status, body, err := w.Sender.Send(ctx, c, nil, WebhookEvent{Type: "inspection.finding." + f.Status, Finding: f, RunID: d.RunID})
	return true, w.Store.FinishDelivery(ctx, d, status, body, err)
}
