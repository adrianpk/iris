package iris

import "fmt"

func (app *App) printStatistics(value statistics) error {
	if _, err := fmt.Fprintf(app.stdout, "Iris ready; schema %d; %d projects; %d conversations; %d messages\n", value.StorageSchemaVersion, value.Projects, value.Conversations, value.Messages); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(app.stdout, "States: pending=%d active=%d closed=%d expired=%d; retained=%d eligible=%d; body bytes=%d\n", value.States.Pending, value.States.Active, value.States.Closed, value.States.Expired, value.RetainedConversations, value.PruneEligible, value.BodyBytes); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(app.stdout, "Policy seconds: invitation=%d inactivity=%d retention=%d\n", value.Lifecycle.InvitationSeconds, value.Lifecycle.InactivitySeconds, value.Lifecycle.RetentionSeconds); err != nil {
		return err
	}

	_, err := fmt.Fprintf(app.stdout, "Service storage: allocated=%d reusable=%d bytes; pages=%d size=%d free=%d\n", value.Storage.AllocatedBytes, value.Storage.ReusableBytes, value.Storage.PageCount, value.Storage.PageSize, value.Storage.FreePages)

	return err
}
