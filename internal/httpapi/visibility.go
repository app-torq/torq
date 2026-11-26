package httpapi

import (
	"context"

	"github.com/app-torq/torq/internal/auth"
)

// visibility is what the signed-in user's role lets them see within the
// vehicles they see (#49). Responses leave hidden values out here, so hiding
// never depends on the app: values that can be null become null, numbers
// that can't become 0, texts "" and lists empty.
type visibility struct {
	costs, notes, attachments, vehicleIDs, reminders, repairs, serviceLogs, reports bool
}

// visibilityOf reads the role from the request context. Without a signed-in
// user (public routes) nothing is hidden here; those build their own records.
func visibilityOf(ctx context.Context) visibility {
	u, ok := auth.UserFrom(ctx)
	if !ok {
		return visibility{true, true, true, true, true, true, true, true}
	}
	return visibilityFor(u.Access)
}

// visibilityFor is visibilityOf for a known access.
func visibilityFor(a auth.Access) visibility {
	return visibility{
		costs: a.Can(auth.PermCostsView), notes: a.Can(auth.PermNotesView), attachments: a.Can(auth.PermAttachmentsView),
		vehicleIDs: a.Can(auth.PermVehicleIDsView), reminders: a.Can(auth.PermRemindersView), repairs: a.Can(auth.PermRepairsView),
		serviceLogs: a.Can(auth.PermServiceLogsView), reports: a.Can(auth.PermReportsView),
	}
}

func (v visibility) serviceLog(j *serviceLogJSON) {
	if !v.costs {
		for i := range j.Items {
			j.Items[i].CostCents = nil
		}
		j.ExtraCostCents, j.ExtraCostLabel, j.TotalCents = 0, "", 0
	}
	if !v.notes {
		j.Notes = ""
	}
	if !v.attachments {
		j.Attachments = nil
	}
}

func (v visibility) repairSummary(j *repairSummaryJSON) {
	if !v.costs {
		j.PartsTotalCents = 0
	}
}

func (v visibility) part(j *partJSON) {
	if !v.costs {
		j.UnitCostCents, j.LineTotalCents = nil, 0
	}
}

func (v visibility) repair(j *repairJSON) {
	v.repairSummary(&j.repairSummaryJSON)
	for i := range j.Parts {
		v.part(&j.Parts[i])
	}
	if !v.notes {
		j.ProcedureNotes = ""
	}
	if !v.attachments {
		j.Attachments = []attachmentJSON{}
	}
}

func (v visibility) vehicle(j *vehicleJSON) {
	if !v.vehicleIDs {
		j.VIN, j.LicensePlate = nil, nil
	}
	if !v.reminders {
		j.ReminderCounts = reminderCounts{}
	}
}

// reminderList hides reminders that come along with other responses (after
// a service entry or an odometer change).
func (v visibility) reminderList(list []reminderSummaryJSON) []reminderSummaryJSON {
	if !v.reminders {
		return []reminderSummaryJSON{}
	}
	return list
}
