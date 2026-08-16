package inspection

import "context"

type CredentialReader interface {
	RevealLinked(context.Context, string) ([]byte, error)
}
type NotificationWorker struct {
	Store       Store
	Credentials CredentialReader
	Sender      WebhookSender
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
	if c.CredentialID == nil {
		return true, w.Store.FinishDelivery(ctx, d, 0, "", invalid("webhook credential is required"))
	}
	secret, err := w.Credentials.RevealLinked(ctx, *c.CredentialID)
	if err != nil {
		return true, w.Store.FinishDelivery(ctx, d, 0, "", err)
	}
	status, body, err := w.Sender.Send(ctx, c, secret, WebhookEvent{Type: "inspection.finding." + f.Status, Finding: f, RunID: d.RunID})
	return true, w.Store.FinishDelivery(ctx, d, status, body, err)
}
