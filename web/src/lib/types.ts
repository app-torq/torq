// API types mirroring contracts/rest-api.md.

export type Locale = 'en' | 'de'
export type DateFormat = 'DD.MM.YYYY' | 'YYYY-MM-DD'
export type NumberFormat = 'point' | 'comma'
export type Role = 'admin' | 'member'

export interface User {
  id: number
  username: string
  display_name: string
  role: Role
  locale: Locale
  date_format: DateFormat
  number_format: NumberFormat
  currency_code: string
  currency_symbol: string | null
  disabled: boolean
}

export interface InstanceInfo {
  max_upload_mb: number
  version: string
}

export interface MeResponse {
  user: User
  instance: InstanceInfo
}

export interface ReminderCounts {
  due_soon: number
  overdue: number
}

export interface Vehicle {
  id: number
  make: string
  model: string
  chassis_code: string
  model_year: number | null
  engine_code: string
  vin: string | null
  license_plate: string | null
  current_odometer_km: number
  avatar_url: string | null
  archived: boolean
  reminder_counts: ReminderCounts
  created_at: string
  updated_at: string
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
}

export interface ServiceItem {
  id: number
  category_id: number
  brand: string
  specification: string
  quantity_ml: number | null
  quantity_unit: 'L' | 'mL' | null
  oem_part_number: string
  aftermarket_part_number: string
  cost_cents: number | null
  cost_type: CostType
}

export interface ServiceLog {
  id: number
  vehicle_id: number
  service_date: string
  odometer_km: number
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
  baseline: { date: string; odometer_km: number; source: 'service_log' | 'mark_done' | 'start' }
  next_due_km: number | null
  next_due_date: string | null
  remaining_km: number | null
  remaining_days: number | null
  state: ReminderState
  state_trigger: 'km' | 'time' | null
  paused: boolean
}

export type RepairStatus = 'planned' | 'in_progress' | 'waiting_on_parts' | 'completed'
export type PartStatus = 'needed' | 'ordered' | 'in_stock' | 'installed'

export interface RepairSummary {
  id: number
  vehicle_id: number
  title: string
  status: RepairStatus
  started_on: string | null
  completed_on: string | null
  completion_odometer_km: number | null
  fault_codes: string[]
  parts_total_cents: number
  part_status_counts: Record<PartStatus, number>
  updated_at: string
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
}

export interface Costs {
  total_cents: number
  by_source: { routine_service_cents: number; self_repair_cents: number }
  by_cost_type: { parts_cents: number; fluids_cents: number; other_cents: number }
  distance_km: number | null
  cost_per_km_cents: number | null
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
}

export interface VehicleHistory {
  schema: string
  vehicle: { id: number; make: string; model: string; chassis_code: string; model_year: number | null; engine_code: string; vin: string | null; license_plate: string | null; current_odometer_km: number }
  specs: Record<string, string | number | null>
  spec_entries: { label: string; value: string }[]
  service_logs: {
    id: number
    service_date: string
    odometer_km: number
    notes?: string
    extra_cost_cents?: number
    extra_cost_label?: string
    total_cents?: number
    items: { category: string; brand: string; specification: string; quantity_ml: number | null; quantity_unit: 'L' | 'mL' | null; oem_part_number: string; aftermarket_part_number: string; cost_cents?: number | null; cost_type: CostType }[]
  }[]
  repairs: {
    id: number
    title: string
    started_on: string | null
    completed_on: string
    completion_odometer_km: number
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
