package httpapi

import (
	"net/http"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/attachments"
	"github.com/app-torq/torq/internal/auth"
)

// routes registers every /api/v1 endpoint. openapi.yaml describes each one;
// the contract tests fail when the two disagree.
func (s *Server) routes() {
	// Setup & auth
	s.route("GET /instance", public, s.instance)
	s.route("GET /instance/logo", public, s.getLogo)
	s.route("GET /shared/{token}", public, s.getSharedRecord)
	s.route("GET /setup/status", public, s.setupStatus)
	s.route("POST /setup", public, s.setup)
	s.route("POST /auth/login", public, s.login)
	s.route("POST /auth/logout", member, s.logout)
	s.route("GET /auth/me", member, s.me)
	s.route("POST /auth/password", member, s.demoLocked(accountsLocked, s.changePassword))
	s.route("PATCH /me/preferences", member, s.updatePreferences)
	s.route("GET /me/sessions", member, s.demoLocked(accountsLocked, s.listSessions))
	s.route("DELETE /me/sessions/{id}", member, s.demoLocked(accountsLocked, s.revokeSession))
	s.route("DELETE /me/sessions", member, s.demoLocked(accountsLocked, s.revokeOtherSessions))

	// Users (admin)
	s.route("GET /users", admin, s.listUsers)
	s.route("POST /users", admin, s.demoLocked(accountsLocked, s.createUser))
	s.route("PATCH /users/{id}", admin, s.demoLocked(accountsLocked, s.updateUser))
	s.route("POST /users/{id}/password", admin, s.demoLocked(accountsLocked, s.resetUserPassword))
	s.route("DELETE /users/{id}", admin, s.demoLocked(accountsLocked, s.deleteUser))
	s.route("GET /users/{id}/vehicle-access", admin, s.getVehicleAccess)
	s.route("PUT /users/{id}/vehicle-access", admin, s.demoLocked(accountsLocked, s.putVehicleAccess))

	// Roles (admin)
	s.route("GET /roles", admin, s.listRoles)
	s.route("POST /roles", admin, s.demoLocked(accountsLocked, s.createRole))
	s.route("PATCH /roles/{id}", admin, s.demoLocked(accountsLocked, s.updateRole))
	s.route("DELETE /roles/{id}", admin, s.demoLocked(accountsLocked, s.deleteRole))

	// Offline vehicle catalog (specs/002-vehicle-catalog-lookup)
	s.route("GET /catalog/kba", member, s.getCatalogKBA)
	s.route("GET /catalog/makes", member, s.getCatalogMakes)
	s.route("GET /catalog/models", member, s.getCatalogModels)
	s.route("GET /catalog/generations", member, s.getCatalogGenerations)
	s.route("GET /catalog/engines", member, s.getCatalogEngines)
	s.route("GET /catalog/meta", member, s.getCatalogMeta)
	s.route("POST /catalog/update", garage(auth.PermCatalogUpdate), s.startCatalogUpdate)

	// Vehicles & odometer
	s.route("GET /vehicles", member, s.listVehicles)
	s.route("POST /vehicles", garage(auth.PermVehiclesCreate), s.createVehicle)
	s.route("GET /vehicles/{id}", onVehicle(""), s.getVehicle)
	s.route("PATCH /vehicles/{id}", onVehicle(auth.PermVehiclesEdit), s.updateVehicle)
	s.route("DELETE /vehicles/{id}", onVehicle(auth.PermVehiclesDelete), s.deleteVehicle)
	s.route("POST /vehicles/{id}/archive", onVehicle(auth.PermVehiclesEdit), s.setVehicleArchived(true))
	s.route("POST /vehicles/{id}/unarchive", onVehicle(auth.PermVehiclesEdit), s.setVehicleArchived(false))
	s.route("POST /vehicles/{id}/odometer", onVehicle(auth.PermVehiclesEdit), s.quickUpdateOdometer)
	s.route("GET /vehicles/{id}/odometer-readings", onVehicle(""), s.listOdometerReadings)
	s.route("POST /vehicles/{id}/hour-meters", onVehicle(auth.PermVehiclesEdit), s.createHourMeter)
	s.route("PATCH /hour-meters/{id}", onVehicle(auth.PermVehiclesEdit), s.renameHourMeter)
	s.route("DELETE /hour-meters/{id}", onVehicle(auth.PermVehiclesEdit), s.deleteHourMeter)
	s.route("POST /hour-meters/{id}/readings", onVehicle(auth.PermVehiclesEdit), s.quickUpdateHours)
	s.route("GET /hour-meters/{id}/readings", onVehicle(""), s.listHourMeterReadings)
	s.route("GET /vehicles/{id}/access", admin, s.listVehicleUsers)
	s.route("GET /vehicles/{id}/share", onVehicle(""), s.getShare)
	s.route("PUT /vehicles/{id}/share", onVehicle(auth.PermSharesManage), s.demoLocked(sharesLocked, s.putShare))
	s.route("POST /vehicles/{id}/share/renew", onVehicle(auth.PermSharesManage), s.demoLocked(sharesLocked, s.renewShare))
	s.route("DELETE /vehicles/{id}/share", onVehicle(auth.PermSharesManage), s.deleteShare)

	// Spec sheet
	s.route("GET /vehicles/{id}/specs", onVehicle(""), s.getSpecs)
	s.route("PUT /vehicles/{id}/specs", onVehicle(auth.PermSpecsEdit), s.putSpecs)
	s.route("POST /vehicles/{id}/autofill-specs", onVehicle(auth.PermSpecsEdit), s.autofillSpecs)
	s.route("POST /vehicles/{id}/spec-entries", onVehicle(auth.PermSpecsEdit), s.createSpecEntry)
	s.route("PUT /vehicles/{id}/spec-entries/order", onVehicle(auth.PermSpecsEdit), s.reorderSpecEntries)
	s.route("PATCH /spec-entries/{id}", onVehicle(auth.PermSpecsEdit), s.updateSpecEntry)
	s.route("DELETE /spec-entries/{id}", onVehicle(auth.PermSpecsEdit), s.deleteSpecEntry)

	// Service categories
	s.route("GET /service-categories", member, s.listCategories)
	s.route("POST /service-categories", garage(auth.PermCategoriesManage), s.createCategory)
	s.route("PATCH /service-categories/{id}", garage(auth.PermCategoriesManage), s.updateCategory)
	s.route("POST /service-categories/{id}/archive", garage(auth.PermCategoriesManage), s.setCategoryArchived(true))
	s.route("POST /service-categories/{id}/unarchive", garage(auth.PermCategoriesManage), s.setCategoryArchived(false))
	s.route("DELETE /service-categories/{id}", garage(auth.PermCategoriesManage), s.deleteCategory)

	// Service logbook
	s.route("GET /service-logs", member, s.listServiceLogs)
	s.route("POST /service-logs", onVehicle(auth.PermServiceLogsWrite), s.createServiceLog)
	s.route("GET /service-logs/{id}", onVehicle(""), s.getServiceLog)
	s.route("PUT /service-logs/{id}", onVehicle(auth.PermServiceLogsWrite), s.replaceServiceLog)
	s.route("DELETE /service-logs/{id}", onVehicle(auth.PermServiceLogsDelete), s.deleteServiceLog)

	// Reminders & dashboard
	s.route("GET /reminders", member, s.listReminders)
	s.route("POST /reminders", onVehicle(auth.PermRemindersWrite), s.createReminder)
	s.route("GET /reminders/{id}", onVehicle(""), s.getReminder)
	s.route("PATCH /reminders/{id}", onVehicle(auth.PermRemindersWrite), s.updateReminder)
	s.route("DELETE /reminders/{id}", onVehicle(auth.PermRemindersWrite), s.deleteReminder)
	s.route("POST /reminders/{id}/done", onVehicle(auth.PermRemindersWrite), s.markReminderDone)
	s.route("POST /reminders/{id}/pause", onVehicle(auth.PermRemindersWrite), s.setReminderPaused(true))
	s.route("POST /reminders/{id}/resume", onVehicle(auth.PermRemindersWrite), s.setReminderPaused(false))
	s.route("GET /dashboard", member, s.dashboard)

	// Self-repair projects
	s.route("GET /repairs", member, s.listRepairs)
	s.route("POST /repairs", onVehicle(auth.PermRepairsWrite), s.createRepair)
	s.route("GET /repairs/{id}", onVehicle(""), s.getRepair)
	s.route("PATCH /repairs/{id}", onVehicle(auth.PermRepairsWrite), s.updateRepair)
	s.route("DELETE /repairs/{id}", onVehicle(auth.PermRepairsDelete), s.deleteRepair)
	s.route("POST /repairs/{id}/status", onVehicle(auth.PermRepairsWrite), s.setRepairStatus)
	s.route("POST /repairs/{id}/parts", onVehicle(auth.PermRepairsWrite), s.createPart)
	s.route("PUT /repairs/{id}/parts/order", onVehicle(auth.PermRepairsWrite), s.reorderParts)
	s.route("PATCH /repair-parts/{id}", onVehicle(auth.PermRepairsWrite), s.updatePart)
	s.route("GET /repair-priorities", member, s.listPriorities)
	s.route("POST /repair-priorities", garage(auth.PermRepairListsManage), s.createPriority)
	s.route("PUT /repair-priorities/order", garage(auth.PermRepairListsManage), s.reorderPriorities)
	s.route("PATCH /repair-priorities/{id}", garage(auth.PermRepairListsManage), s.updatePriority)
	s.route("DELETE /repair-priorities/{id}", garage(auth.PermRepairListsManage), s.deletePriority)
	s.route("GET /repair-tags", member, s.listTags)
	s.route("PATCH /repair-tags/{id}", garage(auth.PermRepairListsManage), s.renameTag)
	s.route("DELETE /repair-tags/{id}", garage(auth.PermRepairListsManage), s.deleteTag)
	s.route("DELETE /repair-parts/{id}", onVehicle(auth.PermRepairsWrite), s.deletePart)

	// Attachments
	s.route("POST /service-logs/{id}/attachments", onVehicle(auth.PermServiceLogsWrite), s.demoLocked(uploadsLocked, s.uploadTo(func(id int64) attachments.Owner { return attachments.Owner{ServiceLogID: id} })))
	s.route("POST /repairs/{id}/attachments", onVehicle(auth.PermRepairsWrite), s.demoLocked(uploadsLocked, s.uploadTo(func(id int64) attachments.Owner { return attachments.Owner{RepairLogID: id} })))
	s.route("POST /spec-entries/{id}/attachments", onVehicle(auth.PermSpecsEdit), s.demoLocked(uploadsLocked, s.uploadTo(func(id int64) attachments.Owner { return attachments.Owner{SpecEntryID: id} })))
	s.route("PUT /vehicles/{id}/avatar", onVehicle(auth.PermVehiclesEdit), s.demoLocked(uploadsLocked, s.putAvatar))
	s.route("DELETE /vehicles/{id}/avatar", onVehicle(auth.PermVehiclesEdit), s.deleteAvatar)
	s.route("GET /attachments/{id}", onVehicle(""), s.getAttachment)
	s.route("PATCH /attachments/{id}", onVehicle(""), s.patchAttachment)
	s.route("DELETE /attachments/{id}", onVehicle(""), s.deleteAttachment)

	// Webhooks & instance settings (admin)
	s.route("GET /webhooks", admin, s.listWebhooks)
	s.route("POST /webhooks", admin, s.demoLocked(webhooksLocked, s.createWebhook))
	s.route("GET /webhooks/{id}", admin, s.getWebhook)
	s.route("PATCH /webhooks/{id}", admin, s.demoLocked(webhooksLocked, s.updateWebhook))
	s.route("DELETE /webhooks/{id}", admin, s.demoLocked(webhooksLocked, s.deleteWebhook))
	s.route("POST /webhooks/{id}/test", admin, s.demoLocked(webhooksLocked, s.testWebhook))
	s.route("GET /webhooks/{id}/deliveries", admin, s.listDeliveries)
	s.route("GET /shares", admin, s.listShares)
	s.route("GET /settings/instance", admin, s.getInstanceSettings)
	s.route("PATCH /settings/instance", admin, s.patchInstanceSettings)
	s.route("PUT /settings/instance/logo", admin, s.demoLocked(brandingLocked, s.putLogo))
	s.route("DELETE /settings/instance/logo", admin, s.demoLocked(brandingLocked, s.deleteLogo))

	// Reports & export
	s.route("GET /reports/costs", member, s.costReport)
	s.route("GET /vehicles/{id}/timeline", onVehicle(""), s.timeline)
	s.route("GET /vehicles/{id}/history", onVehicle(auth.PermHistoryExport), s.history)
}

// Demo mode error codes (see demoLocked).
const (
	accountsLocked = "demo.accounts_locked"
	webhooksLocked = "demo.webhooks_locked"
	uploadsLocked  = "demo.uploads_locked"
	brandingLocked = "demo.branding_locked"
	sharesLocked   = "demo.shares_locked"
)

// demoLocked refuses fn with code in demo mode. Accounts are locked so nobody
// can lock other visitors out of the shared demo login, which also covers the
// Devices list: it would show visitors each other's IP addresses and let them
// sign each other out; webhooks so the
// public demo cannot be used to send requests to arbitrary URLs; uploads so
// visitors cannot store files on the demo server; the instance name and logo
// because every visitor sees them on the sign-in page; public links so the
// demo can't publish anything.
func (s *Server) demoLocked(code string, fn handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if s.Config.DemoMode {
			return apperr.Forbidden(code)
		}
		return fn(w, r)
	}
}
