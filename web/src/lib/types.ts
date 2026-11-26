// API types mirroring contracts/rest-api.md.

export type Locale = 'en' | 'de'
import type { DisplacementUnit, DistanceUnit, PowerUnit, PressureUnit, QuantityUnit, TorqueUnit, VolumeUnit } from './units'

export type { DisplacementUnit, DistanceUnit, PowerUnit, PressureUnit, QuantityUnit, TorqueUnit, VolumeUnit }
export type DateFormat = 'DD.MM.YYYY' | 'YYYY-MM-DD' | 'MM/DD/YYYY'
export type NumberFormat = 'point' | 'comma'
export type TimeFormat = '24h' | '12h'
/** How project priorities are shown. */
export type PriorityDisplay = 'both' | 'color' | 'name'
/** admin for the Administrator role, member for every other (older clients). */
export type LegacyRole = 'admin' | 'member'

/**
 * A permission key. Per-vehicle keys apply to vehicles the user may edit,
 * garage keys everywhere; the list can grow, so unknown keys are possible.
 */
export type Permission =
  | 'vehicles.edit' | 'vehicles.delete' | 'specs.edit' | 'service_logs.write' | 'service_logs.delete'
  | 'reminders.write' | 'repairs.write' | 'repairs.delete' | 'shares.manage' | 'history.export'
  | 'vehicles.create' | 'categories.manage' | 'repair_lists.manage' | 'catalog.update'
  | 'costs.view' | 'service_logs.view' | 'repairs.view' | 'reminders.view' | 'reports.view'
  | 'attachments.view' | 'notes.view' | 'vehicle_ids.view'

export type RoleBuiltinKey = 'admin' | 'member' | 'viewer'

/** A role: a named set of permissions and the vehicles its users see. */
export interface Role {
  id: number
  /** custom_name, or the built-in name in the caller's language. */
  name: string
  custom_name: string | null
  builtin_key: RoleBuiltinKey | null
  description: string
  /** True: every vehicle; false: only vehicles assigned to the user. */
  all_vehicles: boolean
  permissions: Permission[]
  user_count: number
  /** The role's users must have a second factor. */
  require_mfa: boolean
}

export interface Passkey {
  id: number
  name: string
  /** Verifies its user, so it can sign in without a password. */
  user_verified: boolean
  created_at: string
  last_used_at: string | null
}

/** The signed-in user's second factors (GET /me/mfa). */
export interface MfaStatus {
  totp: boolean
  recovery_codes_left: number
  passkeys: Passkey[]
  required: boolean
  passkeys_available: boolean
}

export type MfaMethod = 'totp' | 'recovery_code' | 'passkey'

export interface PermissionInfo {
  key: Permission
  /** view: what the user sees on every visible vehicle (API level 12). */
  scope: 'vehicle' | 'garage' | 'view'
}

export type AccessLevel = 'view' | 'edit'

export interface VehicleAssignment {
  vehicle_id: number
  level: AccessLevel
}

/** A user who can see a vehicle, and how (admin overview). */
export interface VehicleUser {
  user_id: number
  username: string
  display_name: string
  role_id: number
  role_name: string
  via: 'role' | 'assignment'
  level: AccessLevel
}

/** Format and unit preferences; also the instance defaults for new users. */
export interface Formats {
  date_format: DateFormat
  number_format: NumberFormat
  time_format: TimeFormat
  currency_code: string
  power_unit: PowerUnit
  /** For new vehicles and distances not tied to a vehicle. */
  distance_unit: DistanceUnit
  volume_unit: VolumeUnit
  torque_unit: TorqueUnit
  pressure_unit: PressureUnit
  displacement_unit: DisplacementUnit
}

export interface User extends Formats {
  id: number
  username: string
  display_name: string
  role: LegacyRole
  role_id: number
  role_name: string
  role_builtin_key: RoleBuiltinKey | null
  /** Assigned vehicles (admin user list only). */
  vehicle_count?: number
  /** The signed-in user's granted permissions (me, login, setup only). */
  permissions?: Permission[]
  /** Whether the signed-in user sees every vehicle (me, login, setup only). */
  all_vehicles?: boolean
  /** The user has a second factor. */
  mfa_enabled: boolean
  /** An administrator requires a second factor for this user. */
  mfa_required: boolean
  /** The signed-in user must set up a second factor before anything else (me, login, setup only). */
  mfa_setup_required?: boolean
  locale: Locale
  currency_symbol: string | null
  priority_display: PriorityDisplay
  /** Tags a project card shows before "+ n more"; 0 shows all. */
  card_tag_limit: number
  disabled: boolean
}

export interface InstanceInfo {
  /** API level; raised with every release that adds to the API. */
  api_version: number
  max_upload_mb: number
  version: string
  demo_mode: boolean
  /** Sign-in can return a bearer token for apps. */
  token_auth: boolean
  /** The web spec lookup is configured (GEMINI_API_KEY). */
  spec_autofill: boolean
  branding: Branding
  /** Vehicles can be shared by public link on this server. */
  public_links: boolean
  /** Passkeys work on this server (it needs TORQ_BASE_URL). */
  passkeys: boolean
}

/** A vehicle's public read-only link (#38). */
export interface Share {
  token: string
  /** "/s/<token>"; the app prefixes its own address. */
  path: string
  include_costs: boolean
  include_license_plate: boolean
  include_vin: boolean
  created_at: string
  last_opened_at: string | null
}

/** One active link in the administrators' list (no token). */
export interface ShareListItem {
  vehicle_id: number
  vehicle_name: string
  archived: boolean
  created_at: string
  created_by_name: string | null
  last_opened_at: string | null
}

/** What a share link shows to anyone who has it. */
export interface PublicVehicleRecord {
  /** Only for signed-in visitors. */
  vehicle_id?: number
  vehicle: {
    vehicle_type: VehicleType
    make: string
    model: string
    chassis_code: string
    model_year: number | null
    engine_code: string
    hsn: string | null
    tsn: string | null
    displacement_cc: number | null
    power_kw: number | null
    power_w: number | null
    fuel_type: FuelType | null
    has_odometer: boolean
    current_odometer_km: number
    odometer_unit: DistanceUnit
    displacement_unit: DisplacementUnit | null
    power_unit: PowerUnit | null
    hour_meters: HourMeter[]
    license_plate: string | null
    vin: string | null
  }
  service_logs: {
    service_date: string
    odometer_km: number
    odometer_recorded: boolean
    meter_readings: MeterReading[]
    items: {
      category: string
      brand: string
      specification: string
      quantity_ml: number | null
      quantity_unit: QuantityUnit | null
      oem_part_number: string
      aftermarket_part_number: string
      cost_cents: number | null
    }[]
    extra_cost_cents: number | null
    extra_cost_label: string
    total_cents: number | null
  }[]
  reminders: {
    name: string
    state: 'upcoming' | 'due_soon' | 'overdue'
    trigger: string | null
    next_due_date: string | null
    next_due_km: number | null
    next_due_hours: number | null
    remaining_days: number | null
    remaining_km: number | null
    remaining_hours: number | null
    hour_meter: string | null
  }[]
  specs: Record<string, string | number | null>
  spec_entries: { label: string; value: string }[]
  formats: Formats
  branding: { name: string; logo_url: string | null }
}

/** Contact details for the printed history; empty strings when not set. */
export interface Contact {
  address: string
  phone: string
  email: string
  website: string
}

/** Instance name and logo set by an admin. */
export interface Branding {
  /** Empty when not set. */
  name: string
  /** Versioned logo URL, null without a logo. */
  logo_url: string | null
  /** Only for signed-in users. */
  contact?: Contact
}

/** GET /instance, which needs no sign-in. */
export interface ServerInfo extends InstanceInfo {
  setup_required: boolean
}

/** One of the signed-in user's sessions in the Devices list. */
export interface Session {
  id: number
  /** Set by apps at sign-in; empty for browsers. */
  device_name: string
  /** Product names parsed from the user agent, empty when unknown. */
  browser: string
  os: string
  created_at: string
  last_seen_at: string
  ip: string
  current: boolean
}

export interface MeResponse {
  user: User
  instance: InstanceInfo
}

export interface ReminderCounts {
  due_soon: number
  overdue: number
}

export const FUEL_TYPES = ['diesel', 'petrol', 'hybrid', 'electric', 'lpg', 'cng', 'other'] as const
export type FuelType = (typeof FUEL_TYPES)[number]

/** Vehicle types; the API may add more, which the app treats like a car. */
export type VehicleType = 'car' | 'motorcycle'
export const VEHICLE_TYPES: readonly VehicleType[] = ['car', 'motorcycle']

export interface Vehicle {
  id: number
  vehicle_type: VehicleType
  make: string
  model: string
  chassis_code: string
  model_year: number | null
  engine_code: string
  vin: string | null
  license_plate: string | null
  hsn: string | null
  tsn: string | null
  displacement_cc: number | null
  /** Whole kW; power_w is the precise value. */
  power_kw: number | null
  power_w: number | null
  fuel_type: FuelType | null
  current_odometer_km: number
  /** How the odometer reads; distances are always sent in km. */
  odometer_unit: DistanceUnit
  /** false: tracked by hours only; current_odometer_km is then 0 and means nothing. */
  has_odometer: boolean
  /** At most one for now. */
  hour_meters: HourMeter[]
  /** Display units for this vehicle; null follows the user's preference. */
  displacement_unit: DisplacementUnit | null
  power_unit: PowerUnit | null
  avatar_url: string | null
  archived: boolean
  reminder_counts: ReminderCounts
  created_at: string
  updated_at: string
  /** The signed-in user's per-vehicle permissions on it; empty when read only. */
  permissions: Permission[]
}

export interface KbaVariant {
  make: string
  /** Empty when the KBA entry names only the brand. */
  model: string
  trade_name: string
  generation: string
  engine_code: string
  displacement_cc: number | null
  power_kw: number | null
  fuel_type: FuelType | null
  /** EU body code such as "AA" or "AC"; empty when unknown. */
  body_code: string
  year_from: number | null
  match: 'unique' | 'none'
}

export interface CatalogModel {
  name: string
  year_from: number | null
  year_to: number | null
}

export const CATALOG_BODIES = ['sedan', 'hatchback', 'estate', 'coupe', 'cabriolet', 'roadster', 'sportback'] as const
export type CatalogBody = (typeof CATALOG_BODIES)[number]

export interface CatalogGeneration extends CatalogModel {
  /** Chassis code ("W203") or descriptor with years; body is set for chassis codes when all variants share it. */
  body: CatalogBody | ''
}

export interface CatalogEngine {
  name: string
  engine_code: string
  displacement_cc: number | null
  power_kw: number | null
  fuel_type: FuelType | null
  /** Generation years for cars; years of first registration of a motorcycle version. */
  year_from: number | null
  year_to: number | null
}

export interface CatalogDataset {
  id: string
  name: string
  publisher: string
  uri: string
  stand?: string
  retrieved: string
  commit?: string
  license: string
  license_id: string
  license_url: string
  /** Verbatim attribution text required by the license; never translated. */
  attribution: string
  modified: boolean
}

export type CatalogUpdateState = 'idle' | 'running' | 'succeeded' | 'unchanged' | 'failed'

export interface CatalogUpdate {
  /** Online updates are allowed on this server (TORQ_CATALOG_ONLINE_UPDATES). */
  enabled: boolean
  state: CatalogUpdateState
  started_at: string | null
  finished_at: string | null
  error: string | null
}

export interface CatalogMeta {
  imported_at: string | null
  /** Bundled with this release, or fetched by an online update. */
  source: 'bundled' | 'online'
  datasets: CatalogDataset[]
  counts: { kba: number; vehicles: number; motorcycles: number }
  update: CatalogUpdate
}

export interface OdometerReading {
  id: number
  reading_date: string
  odometer_km: number
  source: 'initial' | 'quick_update' | 'service_log' | 'repair_completion' | 'reminder_done'
}

export interface VehicleSpecs {
  engine_oil_viscosity: string
  engine_oil_spec: string
  engine_oil_capacity_ml: number | null
  transmission_fluid_spec: string
  transmission_fluid_capacity_ml: number | null
  differential_fluid_spec: string
  differential_fluid_capacity_ml: number | null
  brake_fluid_spec: string
  brake_fluid_capacity_ml: number | null
  coolant_spec: string
  coolant_capacity_ml: number | null
  wheel_lug_torque_nm: number | null
  tire_pressure_front_bar: number | null
  tire_pressure_rear_bar: number | null
  /** Upper end of a tyre pressure range (normal to full load); null for a single value. */
  tire_pressure_front_max_bar: number | null
  tire_pressure_rear_max_bar: number | null
  /** Provenance per field; fields without an entry were entered by hand. */
  meta: Partial<Record<SpecFieldKey, SpecMeta>>
  motorcycle: MotorcycleSpecs
}

export type FinalDrive = 'chain' | 'belt' | 'shaft'
export const FINAL_DRIVES: readonly FinalDrive[] = ['chain', 'belt', 'shaft']

/** Motorcycle fields of the spec sheet, kept for every vehicle type. */
export interface MotorcycleSpecs {
  final_drive: FinalDrive | null
  sprocket_front_teeth: number | null
  sprocket_rear_teeth: number | null
  /** Free text such as "525" or "520 X-ring". */
  chain_size: string
  chain_links: number | null
  fork_oil_spec: string
  /** Per fork leg. */
  fork_oil_capacity_ml: number | null
  /** Valve clearances in mm; the max is the optional upper end of a range. */
  valve_clearance_intake_min_mm: number | null
  valve_clearance_intake_max_mm: number | null
  valve_clearance_exhaust_min_mm: number | null
  valve_clearance_exhaust_max_mm: number | null
}

export type MotoFieldKey = keyof MotorcycleSpecs

export const SPEC_FIELD_KEYS = [
  'engine_oil_viscosity', 'engine_oil_spec', 'engine_oil_capacity_ml',
  'transmission_fluid_spec', 'transmission_fluid_capacity_ml',
  'differential_fluid_spec', 'differential_fluid_capacity_ml',
  'brake_fluid_spec', 'brake_fluid_capacity_ml',
  'coolant_spec', 'coolant_capacity_ml',
  'wheel_lug_torque_nm', 'tire_pressure_front_bar', 'tire_pressure_rear_bar',
  'tire_pressure_front_max_bar', 'tire_pressure_rear_max_bar',
] as const
export type SpecFieldKey = (typeof SPEC_FIELD_KEYS)[number]

/** Where a spec value came from: the web auto-fill (is_auto) or a manual adjustment of it. */
export interface SpecMeta {
  is_auto: boolean
  source_url?: string
  source_name?: string
}

/** One field of POST /vehicles/{id}/autofill-specs; capacities are keyed *_capacity_l and given in litres. */
export interface AutofillValue {
  value: string | number | null
  /** Upper end of a range (tyre pressure); absent or null for a single value. */
  value_max?: number | null
  source_url: string | null
  source_name: string | null
}

export interface Attachment {
  id: number
  original_name: string
  mime_type: string
  size_bytes: number
  caption: string
  url: string
  created_at: string
}

export interface SpecEntry {
  id: number
  label: string
  value: string
  sort_order: number
  attachments: Attachment[]
}

export interface ListResponse<T> {
  items: T[]
  total: number
}

export function vehicleName(v: Pick<Vehicle, 'make' | 'model'>): string {
  return `${v.make} ${v.model}`
}

export type CostType = 'parts' | 'fluids'

export interface ServiceCategory {
  id: number
  builtin_key: string | null
  custom_name: string | null
  is_fluid: boolean
  default_interval_km: number | null
  default_interval_months: number | null
  archived: boolean
  /** Set when a built-in only applies to one vehicle type; null for all types and for custom categories. */
  vehicle_type: VehicleType | null
}

export interface ServiceItem {
  id: number
  category_id: number
  brand: string
  specification: string
  quantity_ml: number | null
  quantity_unit: QuantityUnit | null
  oem_part_number: string
  aftermarket_part_number: string
  cost_cents: number | null
  cost_type: CostType
}

export interface ServiceLog {
  id: number
  vehicle_id: number
  service_date: string
  /** 0 and meaningless when odometer_recorded is false (vehicle without an odometer). */
  odometer_km: number
  odometer_recorded: boolean
  meter_readings: MeterReading[]
  notes: string
  extra_cost_cents: number
  extra_cost_label: string
  items: ServiceItem[]
  total_cents: number
  attachments?: Attachment[]
  created_at: string
  updated_at: string
}

export type ReminderState = 'upcoming' | 'due_soon' | 'overdue'

export interface Reminder {
  id: number
  vehicle_id: number
  service_category_id: number | null
  title: string
  interval_km: number | null
  interval_months: number | null
  interval_days: number | null
  lead_km: number | null
  lead_days: number | null
  start_date: string
  start_odometer_km: number
  /** Never done or unknown: due at the start point until the work is logged. */
  last_done_unknown: boolean
  /** hours is null for reminders without an hour meter. */
  baseline: { date: string; odometer_km: number; hours: number | null; source: 'service_log' | 'mark_done' | 'start' | 'unknown' }
  next_due_km: number | null
  next_due_date: string | null
  remaining_km: number | null
  remaining_days: number | null
  state: ReminderState
  state_trigger: 'km' | 'time' | 'hours' | null
  paused: boolean
  /** The hour meter of an hours interval; null without one. */
  hour_meter_id: number | null
  interval_hours: number | null
  lead_hours: number | null
  start_hours: number | null
  last_done_hours: number | null
  next_due_hours: number | null
  /** Negative when overdue. */
  remaining_hours: number | null
}

export type RepairStatus = 'planned' | 'in_progress' | 'waiting_on_parts' | 'completed'
export type PartStatus = 'needed' | 'ordered' | 'in_stock' | 'installed'

export interface RepairSummary {
  id: number
  vehicle_id: number
  title: string
  status: RepairStatus
  /** null: no priority. */
  priority_id: number | null
  started_on: string | null
  completed_on: string | null
  completion_odometer_km: number | null
  /** false when completed on a vehicle without an odometer; completion_odometer_km is then 0. */
  completion_odometer_recorded: boolean
  completion_meter_readings: MeterReading[]
  fault_codes: string[]
  /** Tag names, sorted by name. */
  tags: string[]
  parts_total_cents: number
  part_status_counts: Record<PartStatus, number>
  updated_at: string
}

/** A project priority of the garage, in list order. */
export interface RepairPriority {
  id: number
  /** urgent, high, normal or low for the built-ins; shown translated unless renamed. */
  builtin_key: string | null
  custom_name: string | null
  color: string
  sort_order: number
  is_default: boolean
  project_count: number
}

/** An hour meter counting operating hours, next to the odometer or instead of it. */
export interface HourMeter {
  id: number
  name: string
  /** Hours with at most one decimal. */
  current_hours: number
}

/** Hours recorded on a service entry, repair completion or reminder completion. */
export interface MeterReading {
  hour_meter_id: number
  hours: number
}

export interface HourMeterReading {
  id: number
  reading_date: string
  hours: number
  source: OdometerReading['source']
  created_at: string
}

/** A tag of the garage's shared tag list. */
export interface RepairTag {
  id: number
  name: string
  project_count: number
}

export interface TorqueSpec {
  label: string
  value_nm: number
  note: string
}

export interface RepairPart {
  id: number
  name: string
  oem_part_number: string
  aftermarket_brand: string
  aftermarket_part_number: string
  supplier_url: string
  quantity: number
  unit_cost_cents: number | null
  line_total_cents: number
  cost_type: CostType
  status: PartStatus
  sort_order: number
}

export interface Repair extends RepairSummary {
  symptom: string
  root_cause: string
  procedure_notes: string
  special_tools: string
  torque_specs: TorqueSpec[]
  parts: RepairPart[]
  attachments: Attachment[]
  created_at: string
}

export type WebhookPreset = 'discord' | 'ntfy' | 'gotify' | 'home_assistant' | 'generic_json'

export interface WebhookEvents {
  due_soon: boolean
  overdue: boolean
  weekly_digest: boolean
  monthly_digest: boolean
}

export interface Webhook {
  id: number
  name: string
  url: string
  preset: WebhookPreset
  enabled: boolean
  events: WebhookEvents
  headers: { name: string; value_set: boolean }[]
  owner_user_id: number | null
  created_at: string
  updated_at: string
}

export interface DeliveryResult {
  status: 'succeeded' | 'failed' | 'pending'
  status_code: number | null
  error: string | null
  response_excerpt: string
  duration_ms: number
}

export interface Delivery {
  id: number
  event: 'reminder_due_soon' | 'reminder_overdue' | 'digest_weekly' | 'digest_monthly' | 'test'
  reminder_id: number | null
  status: 'pending' | 'succeeded' | 'failed'
  attempt_count: number
  next_attempt_at: string | null
  last_status_code: number | null
  last_error: string
  response_excerpt: string
  duration_ms: number | null
  created_at: string
  completed_at: string | null
}

export interface InstanceSettings {
  weekly_digest_weekday: number
  weekly_digest_hour: number
  monthly_digest_day: number
  monthly_digest_hour: number
  base_url_configured: boolean
  time_zone: string
  max_upload_mb: number
  version: string
  /** Formats and units new users start with. */
  defaults: Formats
  branding: Branding & { contact: Contact }
  /** The switch for public vehicle links. */
  public_links_enabled: boolean
}

export interface Costs {
  total_cents: number
  by_source: { routine_service_cents: number; self_repair_cents: number }
  by_cost_type: { parts_cents: number; fluids_cents: number; other_cents: number }
  distance_km: number | null
  cost_per_km_cents: number | null
  /** Only for vehicles tracked by hours only. */
  hours: number | null
  cost_per_hour_cents: number | null
}

export interface CostReport extends Costs {
  from: string
  to: string
  by_vehicle?: (Costs & { vehicle_id: number })[]
}

export interface TimelineItem {
  kind: 'service' | 'repair'
  id: number
  date: string
  odometer_km: number
  title?: string
  category_ids?: number[]
  total_cents: number
  odometer_recorded: boolean
  meter_readings: MeterReading[]
}

export interface VehicleHistory {
  schema: string
  instance: { name: string }
  vehicle: {
    id: number; vehicle_type: VehicleType; make: string; model: string; chassis_code: string; model_year: number | null; engine_code: string; vin: string | null
    license_plate: string | null; current_odometer_km: number; hsn: string | null; tsn: string | null
    displacement_cc: number | null; power_kw: number | null; power_w: number | null; fuel_type: FuelType | null; odometer_unit: DistanceUnit
    displacement_unit: DisplacementUnit | null; power_unit: PowerUnit | null
    has_odometer: boolean; hour_meters: HourMeter[]
  }
  specs: Record<string, string | number | null>
  spec_entries: { label: string; value: string }[]
  service_logs: {
    id: number
    service_date: string
    odometer_km: number
    odometer_recorded: boolean
    meter_readings: MeterReading[]
    notes?: string
    extra_cost_cents?: number
    extra_cost_label?: string
    total_cents?: number
    items: { category: string; brand: string; specification: string; quantity_ml: number | null; quantity_unit: QuantityUnit | null; oem_part_number: string; aftermarket_part_number: string; cost_cents?: number | null; cost_type: CostType }[]
  }[]
  repairs: {
    id: number
    title: string
    started_on: string | null
    completed_on: string
    completion_odometer_km: number
    completion_odometer_recorded: boolean
    completion_meter_readings: MeterReading[]
    symptom: string
    root_cause: string
    procedure_notes?: string
    special_tools: string
    fault_codes: string[]
    torque_specs: TorqueSpec[]
    parts: { name: string; oem_part_number: string; aftermarket_brand: string; aftermarket_part_number: string; quantity: number; unit_cost_cents?: number | null; cost_type: CostType; status: PartStatus }[]
    parts_total_cents?: number
  }[]
}
