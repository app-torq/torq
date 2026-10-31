# Feature Specification: torq Core Platform

**Feature Branch**: `001-torq-core-platform`

**Created**: 2026-10-30

**Status**: Draft

## Clarifications

### Session 2026-10-30

- Q: Do household users share one garage or own private garages? → A: One shared garage (Option A, adopted as the recommended default when planning began; revisit via `/speckit-clarify` if needed).

**Input**: User description: "Build torq, a self-hosted web application for car enthusiasts to document DIY repairs, track routine service intervals in a dedicated logbook, manage maintenance reminders, analyze repair costs, and trigger webhook notifications. (Full description: local auth with future OIDC readiness; vehicle management with quick-reference spec sheet; separate Service Logbook; Self-Repair & Project Log with parts BOM; reminders engine with odometer/time/composite triggers; webhook notifications with Discord/ntfy/Gotify/Home Assistant/Generic JSON presets; reports & analytics with cost breakdowns, cost per km, timeline and exportable history report; strict dark mode, strict metric units, English/German localization.)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - First Boot, Sign-In & Garage Setup (Priority: P1)

A self-hoster starts torq for the first time, creates the initial administrator account, signs in, and adds their vehicles with a full profile (make, model, chassis code, model year, engine code, optional VIN and license plate, current odometer in km, photo).

**Why this priority**: Nothing else in torq works without an account and at least one vehicle. This is the foundation every other story builds on.

**Independent Test**: Start a fresh instance, complete first-boot setup, sign in, add two vehicles, update one vehicle's odometer from the dashboard, sign out and back in, and confirm all data persisted.

**Acceptance Scenarios**:

1. **Given** a fresh instance with no accounts, **When** the user opens the app, **Then** they are guided through creating the initial administrator account, and no other page is reachable until setup is complete.
2. **Given** setup is complete, **When** anyone opens the app again, **Then** the first-boot setup is no longer offered and the login page is shown.
3. **Given** a signed-in user, **When** they create a vehicle with make "Mercedes-Benz", model "C 220 CDI", chassis code "S203", year 2006, engine code "OM646", and odometer 214,500 km, **Then** the vehicle appears in their garage with those values and a placeholder avatar until a photo is uploaded.
4. **Given** a vehicle at 214,500 km, **When** the user uses the dashboard quick-update to enter 215,200 km, **Then** the vehicle's current odometer becomes 215,200 km.
5. **Given** a signed-in user, **When** they change their password (providing the current one), **Then** the new password is required on next login and their other active sessions are ended.
6. **Given** a signed-in user, **When** they sign out, **Then** their session is ended and protected pages require logging in again.

---

### User Story 2 - Service Logbook with Automatic Reminder Reset (Priority: P1)

The user logs routine maintenance (e.g., an oil change plus air and cabin filter in one session) with date, odometer, fluid/part details, cost, notes and receipt photos. Any reminder linked to those service categories automatically recalculates its next due date and/or km.

**Why this priority**: Interval maintenance is the most frequent and most valuable record a DIY mechanic keeps, and it drives the reminders engine.

**Independent Test**: With one vehicle and an "Engine Oil & Filter every 10,000 km / 12 months" reminder, log an oil service at 215,000 km and confirm the reminder now shows next due at 225,000 km and 12 months after the service date.

**Acceptance Scenarios**:

1. **Given** a vehicle, **When** the user logs a service session on 2026-10-15 at 215,000 km with three items (Engine Oil & Filter: brand "Mobil 1", spec "MB 229.51", 6.5 L, filter part no. "A6511800109"; Air Filter; Cabin Filter), **Then** one logbook entry is saved containing three categorized items, each with its own details and cost.
2. **Given** an "Engine Oil & Filter" reminder every 10,000 km or 12 months, **When** the service above is saved, **Then** the reminder's next due becomes 225,000 km and 2027-10-15, and its state is recomputed immediately.
3. **Given** a vehicle whose current odometer is 214,800 km, **When** a service is logged at 215,000 km, **Then** the vehicle's current odometer becomes 215,000 km.
4. **Given** an existing later oil service at 225,000 km, **When** the user back-fills an older oil service at 205,000 km, **Then** the linked reminder keeps its schedule based on the most recent service and the vehicle's current odometer is not lowered.
5. **Given** a logbook entry, **When** the user attaches a receipt PDF and two photos, **Then** they are stored with the entry and can be viewed or downloaded later.
6. **Given** the user needs a service type that isn't built in (e.g., "DPF Additive"), **When** they create a custom service category, **Then** it can be used for logbook items and reminders like the built-in categories.

---

### User Story 3 - Reminders Engine & Garage Dashboard (Priority: P1)

The user sets up odometer-based, time-based, or whichever-comes-first reminders per vehicle, either linked to a service category or as standalone checks. The dashboard shows every reminder's state (`Upcoming`, `Due Soon`, `Overdue`) and how far away it is in km and/or days.

**Why this priority**: Knowing what is due next is the core reason to keep a logbook at all.

**Independent Test**: Create a composite reminder (every 15,000 km or 24 months, lead threshold 1,000 km / 30 days), move the odometer and the date across the thresholds, and confirm the state moves from `Upcoming` to `Due Soon` to `Overdue`.

**Acceptance Scenarios**:

1. **Given** an odometer reminder due at 225,000 km with a 1,000 km lead threshold, **When** the odometer is updated to 224,100 km from any place in the app (dashboard, service log, repair log), **Then** the reminder immediately shows `Due Soon` with 900 km remaining.
2. **Given** the same reminder, **When** the odometer reaches 225,000 km or more, **Then** the reminder shows `Overdue` and how many km over it is.
3. **Given** a time-based reminder "Brake Fluid every 24 months" last done on 2024-11-10 with a 30-day lead, **When** the date reaches 2026-10-11, **Then** the reminder becomes `Due Soon` without any user action, and becomes `Overdue` on 2026-11-10.
4. **Given** a composite reminder, **When** either the km limit or the time limit is reached first, **Then** the reminder takes the state of whichever limit is more urgent and shows which one triggered it.
5. **Given** a standalone check (e.g., "Inspect CV boots every 20,000 km") with no linked category, **When** the user marks it done, **Then** its next due is recalculated from the date and odometer entered when marking it done.
6. **Given** a reminder linked to a category with no service history yet, **When** it is created, **Then** the user must provide a starting point (last-done date and/or km), which defaults to today and the current odometer.

---

### User Story 4 - Self-Repair & Project Log with Parts BOM (Priority: P2)

The user documents troubleshooting, unscheduled repairs, overhauls and modifications as projects. Each project moves through `Planned`, `In Progress`, `Waiting on Parts` and `Completed`. It captures symptoms (including OBD2 fault codes), root cause, procedure notes, critical torque specs, special tools, a parts bill of materials with per-part status, and attachments.

**Why this priority**: This is torq's main difference from a plain maintenance tracker. It ranks just behind the P1 stories because those already make the app usable on their own.

**Independent Test**: Create a project "Injector seal replacement" with fault code "P0201", add 4 parts in status `Needed`, move parts to `Ordered` then `Installed`, record torque specs and tools, complete it at 216,000 km, and confirm it appears in the vehicle timeline and updates the odometer.

**Acceptance Scenarios**:

1. **Given** a vehicle, **When** the user creates a project with title, symptom "rough idle cold start, P0201, P0300", status `Planned`, **Then** it appears in that vehicle's Self-Repair Log, separate from the Service Logbook.
2. **Given** a project, **When** the user adds BOM lines (part name, OEM number, aftermarket brand & number, supplier URL, quantity, unit cost, status), **Then** the project shows the line totals, a grand total, and how many parts are in each status.
3. **Given** a project with parts in `Needed` or `Ordered`, **When** the user views the project list, **Then** they can filter projects by status and see which projects are waiting on parts.
4. **Given** a project, **When** the user records torque specs (e.g., "Injector clamp bolt — 7 Nm + 90°") and special tools ("E12 Torx", "Injector puller"), **Then** these appear in a dedicated reference section of the project.
5. **Given** a project in `In Progress`, **When** the user sets it to `Completed`, **Then** a completion date (default today) and completion odometer are required, and the vehicle's current odometer is raised if the entered value is higher.
6. **Given** a completed project, **When** the user reopens it to a non-completed status, **Then** it leaves the timeline and cost reports until it is completed again.
7. **Given** a project, **When** the user attaches before/after photos, a PDF manual excerpt, and an invoice, **Then** all attachments are stored and viewable from the project.

---

### User Story 5 - Quick-Reference Spec Sheet (Priority: P2)

Each vehicle has a pinboard of the specs needed at the workbench: oil viscosity, approval and capacity; transmission, differential, brake and coolant fluid specs and capacities; wheel lug torque; front and rear tire pressures; plus any custom key-value specs (including attached images such as belt routing diagrams).

**Why this priority**: High value and low effort, but the logbook and reminders matter more.

**Independent Test**: Fill the spec sheet for a vehicle, add two custom entries (one with an image), and confirm it shows as a single compact reference view.

**Acceptance Scenarios**:

1. **Given** a vehicle, **When** the user enters "5W-30, MB 229.51, 6.5 L" for engine oil and "150 Nm" for wheel lug torque, **Then** the spec sheet shows these values with their metric units.
2. **Given** a spec sheet, **When** the user adds a custom entry "Serpentine belt routing" with an image, and "Oil drain plug — 30 Nm", **Then** both appear on the pinboard in the order the user chose.
3. **Given** a service log entry form for that vehicle, **When** the user picks "Engine Oil & Filter", **Then** the stored oil spec and capacity are shown as a reference hint.

---

### User Story 6 - Webhook Notifications (Priority: P2)

The user configures one or more webhook endpoints, each with a payload preset (Discord, ntfy, Gotify, Home Assistant, Generic JSON), optional custom HTTP headers, and the events it should receive. They can send a test notification and inspect the response.

**Why this priority**: Reminders only help if the user notices them. Webhooks are torq's only way to send notifications.

**Independent Test**: Configure an ntfy endpoint with an `Authorization` header, send a test notification, check the response status and body shown in the app, then push a reminder into `Due Soon` and confirm exactly one notification is delivered.

**Acceptance Scenarios**:

1. **Given** the webhook settings, **When** the user adds an endpoint with URL, preset "Discord", and events "Due Soon" + "Overdue", **Then** the endpoint is saved and listed as active.
2. **Given** an endpoint, **When** the user clicks "Send Test Notification", **Then** a sample message in that preset's format is sent and the response status code, response body (truncated if long), and duration are shown.
3. **Given** an endpoint subscribed to "Due Soon", **When** a reminder moves from `Upcoming` to `Due Soon`, **Then** exactly one notification is sent for that change, naming the vehicle, the reminder, the remaining km/days, and a link back to torq.
4. **Given** a reminder that stays `Overdue` across many re-evaluations, **When** nothing else changes, **Then** no repeat notifications are sent for the same change.
5. **Given** an endpoint subscribed to the weekly digest, **When** the weekly digest time arrives, **Then** one summary listing all `Due Soon` and `Overdue` reminders across the garage is sent. If nothing is due, the digest says so instead of being skipped.
6. **Given** an endpoint that fails (timeout or non-2xx), **When** a notification is sent, **Then** the system retries a limited number of times, records the failure in a delivery log visible to the user, and keeps processing other endpoints.
7. **Given** a custom header carrying a secret token, **When** the endpoint is viewed later, **Then** the secret value is masked and only replaced if the user enters a new value.

---

### User Story 7 - Cost Reports, Timeline & Vehicle History Export (Priority: P3)

The user reviews spending per vehicle or across the garage over a custom date range: Routine Service vs. Self-Repair Projects, Parts vs. Fluids/Consumables, and cost per km. They browse one chronological timeline of all services and completed repairs, and export a vehicle history report as a printable view, JSON, or CSV, for example when selling the car.

**Why this priority**: Valuable, but it only adds value once there are enough records.

**Independent Test**: With seeded service entries and completed projects, select a date range and confirm the totals, breakdowns, and cost per km match a hand calculation. Then export the history as JSON and CSV and open the printable view.

**Acceptance Scenarios**:

1. **Given** services costing €420 and completed repairs costing €1,180 within 2026, **When** the user views the 2026 report for that vehicle, **Then** the total shows €1,600, split €420 Routine Service / €1,180 Self-Repair Projects, and also split by Parts vs. Fluids/Consumables.
2. **Given** odometer readings of 205,000 km at the start of the range and 225,000 km at the end, **When** the range total is €1,600, **Then** cost per km shows €0.08/km.
3. **Given** a range with fewer than two odometer readings, **When** the report is viewed, **Then** cost per km shows as "not enough data" instead of a misleading number.
4. **Given** a vehicle with logbook entries and projects, **When** the user opens the timeline, **Then** services and *completed* repairs appear together in date order, each clearly labeled by type. Non-completed projects are not shown.
5. **Given** a vehicle, **When** the user opens the history report's printable view, **Then** it shows the vehicle profile, the full service and repair history with dates, km, work done and parts, without app navigation, and prints cleanly.
6. **Given** a vehicle, **When** the user exports JSON or CSV, **Then** they get a file with the complete history. The printable view and exports can optionally leave out costs and private notes (useful for a sale).
7. **Given** a garage-wide report, **When** the user selects "All vehicles", **Then** totals add up over all vehicles and a per-vehicle breakdown is shown.

---

### User Story 8 - Localization & Preferences (Priority: P3)

The user picks the interface language (English or German), date format (`DD.MM.YYYY` or `YYYY-MM-DD`), number format (decimal and thousands separators), and currency symbol. All screens, exports and webhook messages follow these choices.

**Why this priority**: Important for the international DIY community, but defaults (English, `YYYY-MM-DD`, €) are usable meanwhile.

**Independent Test**: Switch to German, `DD.MM.YYYY`, comma decimals and "€", then check the dashboard, a service entry, a report and a test webhook message.

**Acceptance Scenarios**:

1. **Given** language set to German, **When** the user browses the app, **Then** all interface text is German, and any untranslated text falls back to English instead of showing raw keys.
2. **Given** date format `DD.MM.YYYY` and German number format, **When** a service on 2026-10-15 at 215000 km costing 89.5 is shown, **Then** it displays as "15.10.2026", "215.000 km", "89,50 €".
3. **Given** any settings, **When** values are entered, **Then** units are always metric (km, L/mL, Nm, bar). No unit picker exists anywhere.

---

### Edge Cases

- **Odometer going backwards**: Entering a km value lower than the vehicle's current odometer in a quick-update asks for confirmation (e.g., correcting a typo). Historical service and repair entries with lower km are accepted and never lower the current odometer.
- **Odometer inconsistent with history**: A service entry whose km is higher than a *later-dated* entry's km triggers a warning, but can still be saved.
- **Huge odometer jump**: A quick-update that more than doubles the odometer or adds over 100,000 km asks for confirmation (to catch extra-digit typos).
- **Deleting the latest service entry** for a category: linked reminders recalculate from the next most recent entry, or from their manual starting point if none remains.
- **Editing a service entry's date or km**: linked reminders recalculate.
- **Deleting a custom service category** that is in use: blocked, but the category can be archived (hidden from new entries, kept on history).
- **Deleting a vehicle**: requires typing the vehicle name to confirm and removes all its logbook entries, projects, reminders, readings and attachments. Archiving a vehicle (e.g., sold) is offered instead: it hides the vehicle from dashboards and reminders while keeping history and exports.
- **Reminder with neither km nor time interval**: cannot be saved.
- **Lead threshold larger than the interval** (e.g., 12,000 km lead on a 10,000 km interval): rejected with a clear message.
- **Several reminder state changes at once** (e.g., a large odometer jump makes three reminders overdue): one notification per reminder per endpoint. No digest batching for event notifications.
- **App offline while a time-based reminder becomes due**: on next start, time-based states are re-evaluated and any missed changes notify once.
- **Webhook endpoint unreachable, or returns HTML/large bodies**: responses are truncated in the delivery log and failures don't block the app.
- **Attachment too large or of an unsupported type**: rejected with a message naming the limit and the allowed types.
- **Repeated failed logins**: further attempts are throttled after repeated failures.
- **Last administrator**: cannot be deleted or demoted.
- **Currency change**: changing the currency symbol only changes display. Stored amounts are not converted.

## Requirements *(mandatory)*

### Functional Requirements

**Authentication & Accounts**

- **FR-001**: On first start with no accounts, the system MUST require creating an initial administrator account (username + password) before any other function is available, and MUST never offer this setup again once an account exists.
- **FR-002**: Users MUST be able to log in with username and password, log out, and change their own password (confirming the current one).
- **FR-003**: Changing a password MUST end all of that user's other active sessions.
- **FR-004**: Sessions MUST expire after a period of inactivity (default 30 days) and MUST be revocable.
- **FR-005**: Passwords MUST be at least 10 characters and MUST be stored only in a non-reversible form.
- **FR-006**: The system MUST throttle repeated failed login attempts per username and source.
- **FR-007**: Administrators MUST be able to create, disable and delete additional household user accounts, and reset their passwords. The last administrator MUST NOT be deletable or demotable.
- **FR-008**: Each user MUST have an internal identifier that is independent of how they log in. Login credentials MUST be stored as separate linked identities, so an external identity (issuer + subject) can later be linked to an existing user without changing any other stored data.
- **FR-009**: All household users MUST share one garage: every signed-in user can view and edit every vehicle and record. Only administrators can manage user accounts and webhook endpoints.

**Vehicles & Odometer**

- **FR-010**: Users MUST be able to create, view, edit, archive, unarchive and delete vehicles with: make, model, generation/chassis code, model year, engine code, VIN (optional, 17 characters when given), license plate (optional), current odometer (km, whole numbers), and a photo (optional; a placeholder is shown otherwise).
- **FR-011**: The system MUST keep a dated history of odometer readings per vehicle, recorded from quick-updates, service entries and completed repairs.
- **FR-012**: The current odometer MUST be raised whenever a newer, higher reading is recorded from any source (dashboard quick-update, service log, repair completion), and MUST NOT be lowered automatically by historical entries.
- **FR-013**: Lowering the current odometer through a quick-update MUST require explicit confirmation.

**Quick-Reference Spec Sheet**

- **FR-014**: Each vehicle MUST have a spec sheet with structured fields for engine oil (viscosity, specification/approval, capacity in L); transmission, differential, brake and coolant fluids (specification, capacity in L); wheel lug torque (Nm); and tire pressures front/rear (bar).
- **FR-015**: Users MUST be able to add, edit, reorder and remove custom spec entries (label + value, with optional image attachment).
- **FR-016**: Relevant spec sheet values MUST be shown as reference hints when logging a service of the matching category.

**Service Logbook**

- **FR-017**: The system MUST provide these built-in service categories: Engine Oil & Filter, Transmission Fluid, Brake Fluid, Coolant, Differential Fluid, Air Filter, Cabin Filter, Fuel Filter, Spark/Glow Plugs, Brake Pads/Rotors, Timing Belt/Chain. Users MUST be able to create, rename and archive custom categories.
- **FR-018**: A service logbook entry MUST record vehicle, date, odometer (km), notes, attachments, and one or more service items. Each item MUST record a category and MAY record brand, specification used, quantity (L or mL), OEM part number, aftermarket part number, cost, and cost type (Parts or Fluids/Consumables). An entry MAY also record extra costs not tied to an item (e.g., shop supplies).
- **FR-019**: Saving, editing or deleting a service entry MUST recalculate every reminder linked to the categories involved, always basing the next due point on the most recent service of that category for that vehicle (by date, then km).
- **FR-020**: The Service Logbook MUST be a separate area from the Self-Repair Log in both navigation and records. A service entry can never become a repair project, or the other way around.
- **FR-021**: Users MUST be able to list, filter (by vehicle, category, date range) and search service entries.

**Self-Repair & Project Log**

- **FR-022**: A repair project MUST record title, vehicle, status (`Planned`, `In Progress`, `Waiting on Parts`, `Completed`), date started, date completed, odometer at completion (km), symptom/fault description, OBD2 fault codes (zero or more), root cause, repair procedure notes, critical torque specs (label + value in Nm, with optional free-text details such as angle), special tools used, and attachments.
- **FR-023**: Users MUST be able to move a project freely between statuses. Moving to `Completed` MUST require a completion date and completion odometer.
- **FR-024**: Each project MUST support a parts bill of materials. Each line has part name, OEM part number, aftermarket brand, aftermarket part number, supplier/shop URL, quantity, unit cost, cost type (Parts or Fluids/Consumables, default Parts), and status (`Needed`, `Ordered`, `In Stock`, `Installed`).
- **FR-025**: Projects MUST show line totals, a project total, and how many parts are in each status. The project list MUST be filterable by vehicle and status, and searchable (including by fault code and part number).
- **FR-026**: Only `Completed` projects MUST count toward the timeline and cost reports, dated by their completion date.

**Reminders**

- **FR-027**: Users MUST be able to create reminders per vehicle, either linked to a service category or standalone (free-text title).
- **FR-028**: A reminder MUST have a km interval, a time interval (days or months), or both. With both, the reminder is due when either limit is reached, whichever comes first.
- **FR-029**: Each reminder MUST have a lead threshold for `Due Soon` (km and/or days, defaults 1,000 km and 30 days), which must be smaller than the matching interval.
- **FR-030**: Reminder state MUST be computed as `Overdue` once any limit is reached, `Due Soon` once within any lead threshold, otherwise `Upcoming`. The display MUST show remaining or exceeded km and days.
- **FR-031**: Reminder states MUST be re-evaluated immediately when a vehicle's odometer changes or linked services change, and at least once per day for time-based limits (including right after the application starts).
- **FR-032**: Standalone reminders MUST be resettable with a "Mark done" action that records date and km (defaulting to today and the current odometer).
- **FR-033**: Users MUST be able to pause or delete reminders. Reminders of archived vehicles MUST be inactive.
- **FR-034**: The dashboard MUST show all `Overdue` and `Due Soon` reminders across the garage, most urgent first, plus each vehicle's odometer quick-update.

**Webhook Notifications**

- **FR-035**: Notifications MUST be delivered only via outbound HTTP webhooks. The system MUST NOT include email delivery.
- **FR-036**: Users MUST be able to create, edit, enable/disable and delete multiple webhook endpoints. Each has a name, URL, payload preset (Discord, ntfy, Gotify, Home Assistant, Generic JSON), optional custom HTTP headers, and subscribed events (Reminder enters `Due Soon`, Reminder enters `Overdue`, Weekly Digest, Monthly Digest).
- **FR-037**: Each preset MUST produce a payload that the target service accepts and displays correctly without further configuration (title, message, priority/urgency where supported, link back to torq). Generic JSON MUST include structured fields (event type, vehicle, reminder, state, remaining km/days, due values, timestamp).
- **FR-038**: Each reminder state change MUST cause exactly one notification per subscribed endpoint. Repeated evaluations without a change MUST NOT re-notify. A reminder that leaves and later re-enters a state MUST notify again.
- **FR-039**: Weekly digests MUST be sent once per week and monthly digests once per month, at a configurable day and time, summarizing all `Due Soon` and `Overdue` reminders.
- **FR-040**: "Send Test Notification" MUST send a sample payload and show the response status code, response body (truncated to a reasonable length), and elapsed time, or the connection error.
- **FR-041**: Failed deliveries MUST be retried a limited number of times with increasing delays. Every attempt MUST be recorded in a delivery log viewable per endpoint (last 100 entries kept per endpoint).
- **FR-042**: Header values MUST be masked after saving and never shown again in full.
- **FR-043**: Notification text MUST be generated in the language and formats of the user who configured the endpoint (see FR-052).

**Reports & Analytics**

- **FR-044**: Users MUST be able to view spending for one vehicle or all vehicles over a custom date range (with presets: this year, last year, last 12 months, all time), broken down by Routine Service vs. Self-Repair Projects and by Parts vs. Fluids/Consumables (uncategorized extra costs shown separately as "Other").
- **FR-045**: The system MUST compute cost per km for the range as total spending divided by the km driven in that range (difference between the earliest and latest odometer readings in the range), and MUST show "not enough data" when fewer than two readings exist or the distance is zero.
- **FR-046**: Each vehicle MUST have a chronological timeline combining service logbook entries and completed repair projects, each clearly labeled by type and linking to the full record.
- **FR-047**: Each vehicle MUST have a Vehicle History Report with a clean printable view (no app navigation) and downloadable JSON and CSV exports with the vehicle profile, service history (all items) and completed repairs (including BOM). Costs and notes MUST be optional to include.
- **FR-048**: Reports MUST use the single configured currency. No currency conversion is performed.

**UI, Units & Localization**

- **FR-049**: The interface MUST use one high-contrast dark theme. There MUST be no theme toggle, light mode, or custom styling input.
- **FR-050**: All quantities MUST be entered, stored and displayed in metric units only: distance in km, volume in L or mL, torque in Nm, pressure in bar. No unit conversion or unit selection may exist.
- **FR-051**: The interface MUST ship in English and German (de-DE), with English used whenever a translation is missing.
- **FR-052**: Users MUST be able to set interface language, date format (`DD.MM.YYYY` or `YYYY-MM-DD`), number format (decimal comma or decimal point with matching thousands separator), and currency symbol (at least €, $, £, CHF, plus a free-text symbol). Display language and formats are per user. Webhook notifications and digests use the settings of the administrator who configured the endpoint.
- **FR-053**: The interface MUST be usable on phone-sized screens in the garage as well as on desktop.

**Attachments & Data**

- **FR-054**: Attachments (images and PDFs) MUST be supported on service entries, repair projects, custom spec entries, and as vehicle photos, with a configurable maximum size per file (default 25 MB).
- **FR-055**: All user data (records and attachments) MUST be stored on the self-hosted instance, so backing it up only means copying its data storage.

### Key Entities

- **User**: A household member with an internal identifier, display name, role (administrator or member), active/disabled state, and preferences (language, date format, number format, currency symbol).
- **Identity**: A way for a user to log in, linked to exactly one user. In v1, a local username + password credential. Later, an external identity (issuer + subject).
- **Session**: An authenticated login for a user. It expires and can be revoked.
- **Vehicle**: Make, model, chassis code, model year, engine code, VIN, license plate, current odometer, photo, archived flag.
- **Odometer Reading**: A dated km value for a vehicle, with its source (quick-update, service entry, repair completion).
- **Spec Sheet**: One per vehicle. Structured fluid, torque and tire pressure fields plus ordered custom spec entries (label, value, optional image).
- **Service Category**: Built-in or custom type of routine maintenance. Custom categories can be archived.
- **Service Log Entry**: One service session for a vehicle (date, km, notes, extra costs, attachments) containing one or more **Service Items** (category, brand, spec, quantity + unit, part numbers, cost, cost type).
- **Repair Project**: An unscheduled repair/overhaul/modification for a vehicle with status, dates, completion km, diagnostics (symptom, fault codes, root cause, procedure), torque specs, special tools, and attachments. Contains **BOM Lines** (part identity, supplier URL, quantity, unit cost, cost type, status).
- **Reminder**: Per vehicle, linked to a service category or standalone. km and/or time interval, lead thresholds, starting point, computed next due km/date, current state, paused flag.
- **Webhook Endpoint**: Name, URL, preset, masked headers, subscribed events, enabled flag, digest schedule.
- **Notification Delivery**: Record of an attempt to an endpoint (event, time, attempt number, status code or error, truncated response).
- **Attachment**: An uploaded file (image or PDF) with name, type, size and upload date, belonging to one owning record.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new self-hoster can go from first start to a signed-in garage with one fully profiled vehicle in under 5 minutes.
- **SC-002**: A typical multi-item service session (oil + 2 filters, with one photo) can be logged in under 2 minutes, including on a phone.
- **SC-003**: After any odometer change or service save, 100% of affected reminders show the updated state the next time the user looks, with no manual refresh step.
- **SC-004**: Time-based reminders change state within 24 hours of crossing a threshold, even when nobody uses the app.
- **SC-005**: Each reminder state change produces exactly one notification per subscribed endpoint (zero duplicates over 30 days of continuous operation in testing). Reachable endpoints receive it within 5 minutes.
- **SC-006**: Notifications sent with each of the five presets display correctly in the target service on the first try, without extra configuration.
- **SC-007**: Cost totals, breakdowns and cost per km in reports match a hand calculation from the same records exactly (to the cent).
- **SC-008**: A vehicle's complete history (up to 500 records) can be exported or opened in the printable view in under 10 seconds, and the exported file contains every service item and completed repair.
- **SC-009**: Switching to German changes 100% of visible interface text with no raw translation keys visible on any screen.
- **SC-010**: Garage pages and the dashboard load in under 2 seconds on modest home-server hardware (e.g., a single-board computer) with 10 vehicles and 5 years of history.
- **SC-011**: A user can find a past repair's torque spec or part number via search in under 30 seconds.

## Assumptions

- **Users**: The audience is a single person or a small household (up to roughly 5 accounts and 20 vehicles). Multi-tenant fleet use is out of scope.
- **Constitution alignment**: Cost reports are included because they help document the work and support vehicle sale/archival. They stay limited to maintenance and repair spending (no budgets, invoicing, or tax features). Fuel, insurance, registration, inspection and tax tracking are out of scope.
- **Service vs. repair separation**: Routine services and repair projects are separate records and separate areas of the app. Only the timeline and reports combine them, read-only.
- **Cost types**: Each cost line is classified as Parts or Fluids/Consumables. Service items default to Fluids/Consumables for fluid categories (oil, transmission, brake, coolant, differential) and Parts otherwise. BOM lines default to Parts. Users can override any line.
- **Reminder baseline**: The next due point is the last service (date and km) plus the interval. It is not based on the previous due point.
- **Months**: "Every N months" means the same day N months later, clamped to the last day of shorter months.
- **Digests**: Default to Monday 08:00 (weekly) and the 1st of the month 08:00 (monthly), in the server's configured time zone.
- **Webhook retries**: Default to 3 retries over roughly 15 minutes. Endpoints may point at local network addresses (common for Home Assistant, Gotify, ntfy self-hosters).
- **Notification links**: Links back to torq use an externally reachable base URL set by the administrator. Without it, links are left out.
- **Attachments**: Images (JPEG, PNG, WebP, HEIC) and PDFs are supported. Other file types are out of scope for v1.
- **Data import**: Importing from other trackers or spreadsheets is out of scope for v1. JSON export is designed so a future import is possible.
- **Backups**: Built-in backup/restore is out of scope. Backup means copying the instance's data storage.
- **OIDC**: Only the data-model readiness (FR-008) is in scope. No OIDC login is delivered in v1.
- **Self-registration and password recovery by email**: Not provided. Administrators create accounts and reset passwords.
