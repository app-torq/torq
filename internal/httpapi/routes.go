package httpapi

import (
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/attachments"
)

// routes registers every /api/v1 endpoint (contracts/rest-api.md).
func (s *Server) routes() {
	// Setup & auth
	s.route("GET /setup/status", public, s.setupStatus)
	s.route("POST /setup", public, s.setup)
	s.route("POST /auth/login", public, s.login)
	s.route("POST /auth/logout", member, s.logout)
	s.route("GET /auth/me", member, s.me)
	s.route("POST /auth/password", member, s.demoLocked(accountsLocked, s.changePassword))
	s.route("PATCH /me/preferences", member, s.updatePreferences)

	// Users (admin)
	s.route("GET /users", admin, s.listUsers)
	s.route("POST /users", admin, s.demoLocked(accountsLocked, s.createUser))
	s.route("PATCH /users/{id}", admin, s.demoLocked(accountsLocked, s.updateUser))
	s.route("POST /users/{id}/password", admin, s.demoLocked(accountsLocked, s.resetUserPassword))
	s.route("DELETE /users/{id}", admin, s.demoLocked(accountsLocked, s.deleteUser))

	// Offline vehicle catalog (specs/002-vehicle-catalog-lookup)
	s.route("GET /catalog/kba", member, s.getCatalogKBA)
	s.route("GET /catalog/makes", member, s.getCatalogMakes)
	s.route("GET /catalog/models", member, s.getCatalogModels)
	s.route("GET /catalog/generations", member, s.getCatalogGenerations)
	s.route("GET /catalog/engines", member, s.getCatalogEngines)
	s.route("GET /catalog/meta", member, s.getCatalogMeta)
	s.route("POST /catalog/update", admin, s.startCatalogUpdate)

	// Vehicles & odometer
	s.route("GET /vehicles", member, s.listVehicles)
	s.route("POST /vehicles", member, s.createVehicle)
	s.route("GET /vehicles/{id}", member, s.getVehicle)
	s.route("PATCH /vehicles/{id}", member, s.updateVehicle)
	s.route("DELETE /vehicles/{id}", member, s.deleteVehicle)
	s.route("POST /vehicles/{id}/archive", member, s.setVehicleArchived(true))
	s.route("POST /vehicles/{id}/unarchive", member, s.setVehicleArchived(false))
	s.route("POST /vehicles/{id}/odometer", member, s.quickUpdateOdometer)
	s.route("GET /vehicles/{id}/odometer-readings", member, s.listOdometerReadings)

	// Spec sheet
	s.route("GET /vehicles/{id}/specs", member, s.getSpecs)
	s.route("PUT /vehicles/{id}/specs", member, s.putSpecs)
	s.route("POST /vehicles/{id}/autofill-specs", member, s.autofillSpecs)
	s.route("POST /vehicles/{id}/spec-entries", member, s.createSpecEntry)
	s.route("PUT /vehicles/{id}/spec-entries/order", member, s.reorderSpecEntries)
	s.route("PATCH /spec-entries/{id}", member, s.updateSpecEntry)
	s.route("DELETE /spec-entries/{id}", member, s.deleteSpecEntry)

	// Service categories
	s.route("GET /service-categories", member, s.listCategories)
	s.route("POST /service-categories", member, s.createCategory)
	s.route("PATCH /service-categories/{id}", member, s.updateCategory)
	s.route("POST /service-categories/{id}/archive", member, s.setCategoryArchived(true))
	s.route("POST /service-categories/{id}/unarchive", member, s.setCategoryArchived(false))
	s.route("DELETE /service-categories/{id}", member, s.deleteCategory)

	// Service logbook
	s.route("GET /service-logs", member, s.listServiceLogs)
	s.route("POST /service-logs", member, s.createServiceLog)
	s.route("GET /service-logs/{id}", member, s.getServiceLog)
	s.route("PUT /service-logs/{id}", member, s.replaceServiceLog)
	s.route("DELETE /service-logs/{id}", member, s.deleteServiceLog)

	// Reminders & dashboard
	s.route("GET /reminders", member, s.listReminders)
	s.route("POST /reminders", member, s.createReminder)
	s.route("GET /reminders/{id}", member, s.getReminder)
	s.route("PATCH /reminders/{id}", member, s.updateReminder)
	s.route("DELETE /reminders/{id}", member, s.deleteReminder)
	s.route("POST /reminders/{id}/done", member, s.markReminderDone)
	s.route("POST /reminders/{id}/pause", member, s.setReminderPaused(true))
	s.route("POST /reminders/{id}/resume", member, s.setReminderPaused(false))
	s.route("GET /dashboard", member, s.dashboard)

	// Self-repair projects
	s.route("GET /repairs", member, s.listRepairs)
	s.route("POST /repairs", member, s.createRepair)
	s.route("GET /repairs/{id}", member, s.getRepair)
	s.route("PATCH /repairs/{id}", member, s.updateRepair)
	s.route("DELETE /repairs/{id}", member, s.deleteRepair)
	s.route("POST /repairs/{id}/status", member, s.setRepairStatus)
	s.route("POST /repairs/{id}/parts", member, s.createPart)
	s.route("PUT /repairs/{id}/parts/order", member, s.reorderParts)
	s.route("PATCH /repair-parts/{id}", member, s.updatePart)
	s.route("DELETE /repair-parts/{id}", member, s.deletePart)

	// Attachments
	s.route("POST /service-logs/{id}/attachments", member, s.demoLocked(uploadsLocked, s.uploadTo(func(id int64) attachments.Owner { return attachments.Owner{ServiceLogID: id} })))
	s.route("POST /repairs/{id}/attachments", member, s.demoLocked(uploadsLocked, s.uploadTo(func(id int64) attachments.Owner { return attachments.Owner{RepairLogID: id} })))
	s.route("POST /spec-entries/{id}/attachments", member, s.demoLocked(uploadsLocked, s.uploadTo(func(id int64) attachments.Owner { return attachments.Owner{SpecEntryID: id} })))
	s.route("PUT /vehicles/{id}/avatar", member, s.demoLocked(uploadsLocked, s.putAvatar))
	s.route("DELETE /vehicles/{id}/avatar", member, s.deleteAvatar)
	s.route("GET /attachments/{id}", member, s.getAttachment)
	s.route("PATCH /attachments/{id}", member, s.patchAttachment)
	s.route("DELETE /attachments/{id}", member, s.deleteAttachment)

	// Webhooks & instance settings (admin)
	s.route("GET /webhooks", admin, s.listWebhooks)
	s.route("POST /webhooks", admin, s.demoLocked(webhooksLocked, s.createWebhook))
	s.route("GET /webhooks/{id}", admin, s.getWebhook)
	s.route("PATCH /webhooks/{id}", admin, s.demoLocked(webhooksLocked, s.updateWebhook))
	s.route("DELETE /webhooks/{id}", admin, s.demoLocked(webhooksLocked, s.deleteWebhook))
	s.route("POST /webhooks/{id}/test", admin, s.demoLocked(webhooksLocked, s.testWebhook))
	s.route("GET /webhooks/{id}/deliveries", admin, s.listDeliveries)
	s.route("GET /settings/instance", admin, s.getInstanceSettings)
	s.route("PATCH /settings/instance", admin, s.patchInstanceSettings)

	// Reports & export
	s.route("GET /reports/costs", member, s.costReport)
	s.route("GET /vehicles/{id}/timeline", member, s.timeline)
	s.route("GET /vehicles/{id}/history", member, s.history)
}

// Demo mode error codes (see demoLocked).
const (
	accountsLocked = "demo.accounts_locked"
	webhooksLocked = "demo.webhooks_locked"
	uploadsLocked  = "demo.uploads_locked"
)

// demoLocked refuses fn with code in demo mode. Accounts are locked so nobody
// can lock other visitors out of the shared demo login; webhooks so the
// public demo cannot be used to send requests to arbitrary URLs; uploads so
// visitors cannot store files on the demo server.
func (s *Server) demoLocked(code string, fn handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if s.Config.DemoMode {
			return apperr.Forbidden(code)
		}
		return fn(w, r)
	}
}
