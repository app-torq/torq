// Editable state of the "Edit data" spec dialog: the input texts plus, per
// field, where the value came from (web auto-fill or a manual adjustment).
// Pure functions so the rules are easy to test; nothing is saved until the
// user saves the dialog.
import { metricField, parseNumber, readMetricField, toInputValue, type MetricField } from "../../lib/format";
import {
  PRESSURE,
  TORQUE,
  VOLUME,
  type PressureUnit,
  type TorqueUnit,
  type UnitSpec,
  type VolumeUnit,
} from "../../lib/units";
import {
  SPEC_FIELD_KEYS,
  type AutofillValue,
  type MotoFieldKey,
  type MotorcycleSpecs,
  type NumberFormat,
  type SpecFieldKey,
  type SpecMeta,
  type VehicleSpecs,
} from "../../lib/types";

export interface SpecsDraft {
  values: Record<SpecFieldKey, string>;
  meta: Partial<Record<SpecFieldKey, SpecMeta>>;
  /** The text an auto-filled field had when it was filled; typing it back restores the source. */
  auto: Partial<Record<SpecFieldKey, { value: string; meta: SpecMeta }>>;
  /** Saved numbers and their texts: an untouched field is saved unchanged. */
  stored: Partial<Record<SpecFieldKey, MetricField>>;
  /** Motorcycle fields (specs/006-motorcycles); never auto-filled. */
  moto: Record<MotoFieldKey, string>;
  /** The saved fork oil quantity, so an untouched value is saved unchanged. */
  forkOil: MetricField;
}

const MOTO_TEXT: readonly MotoFieldKey[] = ["final_drive", "chain_size", "fork_oil_spec"];
const MOTO_INTEGER: readonly MotoFieldKey[] = ["sprocket_front_teeth", "sprocket_rear_teeth", "chain_links"];
const MOTO_MM: readonly MotoFieldKey[] = [
  "valve_clearance_intake_min_mm",
  "valve_clearance_intake_max_mm",
  "valve_clearance_exhaust_min_mm",
  "valve_clearance_exhaust_max_mm",
];

/** Input texts of the motorcycle fields; fork oil in the user's volume unit, clearances in mm. */
function motoValues(m: MotorcycleSpecs, nf: NumberFormat, forkOil: MetricField): Record<MotoFieldKey, string> {
  const out = {} as Record<MotoFieldKey, string>;
  for (const k of MOTO_TEXT) out[k] = (m[k] as string | null) ?? "";
  for (const k of MOTO_INTEGER) out[k] = m[k] == null ? "" : String(m[k]);
  for (const k of MOTO_MM) out[k] = toInputValue(m[k] as number | null, nf, 2);
  out.fork_oil_capacity_ml = forkOil.text;
  return out;
}

/** The units the dialog shows capacities, torque and pressure in. */
export interface SpecUnits {
  volume: VolumeUnit;
  torque: TorqueUnit;
  pressure: PressureUnit;
}

export const METRIC_SPEC_UNITS: SpecUnits = {
  volume: "l",
  torque: "nm",
  pressure: "bar",
};

export interface AutofillSummary {
  filled: SpecFieldKey[];
  /** Found, but the field already had a value and overwriting was off. */
  kept: SpecFieldKey[];
  /** No value with a reliable source. */
  missing: SpecFieldKey[];
}

const isCapacity = (k: SpecFieldKey) => k.endsWith("_capacity_ml");
const isText = (k: SpecFieldKey) =>
  k === "engine_oil_viscosity" || k.endsWith("_spec");
/** Capacities are stored in mL, torque in Nm and pressure in bar. */
function unitSpec(k: SpecFieldKey, units: SpecUnits): UnitSpec {
  if (isCapacity(k)) return VOLUME[units.volume];
  if (k === "wheel_lug_torque_nm") return TORQUE[units.torque];
  return PRESSURE[units.pressure];
}

/** Input text for a stored (metric) number. */
function inputText(
  k: SpecFieldKey,
  n: number | null,
  nf: NumberFormat,
  units: SpecUnits,
): string {
  return metricField(n, unitSpec(k, units), nf).text;
}

export function initialDraft(
  specs: VehicleSpecs,
  nf: NumberFormat,
  units: SpecUnits = METRIC_SPEC_UNITS,
): SpecsDraft {
  const values = {} as Record<SpecFieldKey, string>;
  const auto: SpecsDraft["auto"] = {};
  const stored: SpecsDraft["stored"] = {};
  for (const k of SPEC_FIELD_KEYS) {
    const v = specs[k];
    if (typeof v === "string") {
      values[k] = v;
    } else {
      stored[k] = metricField(v, unitSpec(k, units), nf);
      values[k] = stored[k].text;
    }
    const m = specs.meta[k];
    if (m?.is_auto && values[k] !== "") auto[k] = { value: values[k], meta: m };
  }
  const forkOil = metricField(specs.motorcycle.fork_oil_capacity_ml, VOLUME[units.volume], nf);
  return { values, meta: { ...specs.meta }, auto, stored, moto: motoValues(specs.motorcycle, nf, forkOil), forkOil };
}

/** A change of a motorcycle field. */
export function editMoto(d: SpecsDraft, k: MotoFieldKey, text: string): SpecsDraft {
  return { ...d, moto: { ...d.moto, [k]: text } };
}

/**
 * A manual edit. Changing an auto-filled value marks the field as adjusted by
 * hand (is_auto false, source dropped); the other fields keep their sources.
 */
export function editField(
  d: SpecsDraft,
  k: SpecFieldKey,
  text: string,
): SpecsDraft {
  const values = { ...d.values, [k]: text };
  const found = d.auto[k];
  if (!found && !d.meta[k]) return { ...d, values };
  const meta: SpecsDraft["meta"] = Object.fromEntries(
    Object.entries(d.meta).filter(([key]) => key !== k),
  );
  if (found && text.trim() === found.value.trim()) meta[k] = found.meta;
  else if (text.trim() !== "") meta[k] = { is_auto: false };
  return { ...d, values, meta };
}

/** The upper-bound field of a tyre pressure range, keyed by its lower-bound field. */
export const RANGE_MAX: Partial<Record<SpecFieldKey, SpecFieldKey>> = {
  tire_pressure_front_bar: "tire_pressure_front_max_bar",
  tire_pressure_rear_bar: "tire_pressure_rear_max_bar",
};
const rangeMaxKeys = new Set<SpecFieldKey>(Object.values(RANGE_MAX));

/**
 * The badge state of a range: auto while every filled end is auto-filled,
 * otherwise the manual marker of whichever end was changed.
 */
export function rangeMeta(
  d: SpecsDraft,
  lo: SpecFieldKey,
  hi: SpecFieldKey,
): SpecMeta | undefined {
  const metas = [d.meta[lo], d.meta[hi]].filter(
    (m): m is SpecMeta => m !== undefined,
  );
  return metas.find((m) => !m.is_auto) ?? metas[0];
}

/** Applies an auto-fill answer; with overwrite off, fields that have a value stay untouched. */
export function applyAutofill(
  d: SpecsDraft,
  fields: Record<string, AutofillValue | undefined>,
  overwrite: boolean,
  nf: NumberFormat,
  units: SpecUnits = METRIC_SPEC_UNITS,
): { draft: SpecsDraft; summary: AutofillSummary } {
  const draft: SpecsDraft = {
    ...d,
    values: { ...d.values },
    meta: { ...d.meta },
    auto: { ...d.auto },
  };
  // Capacities arrive in litres, torque in Nm and pressure in bar.
  const metric = (k: SpecFieldKey, n: number) => (isCapacity(k) ? n * 1000 : n);
  const summary: AutofillSummary = { filled: [], kept: [], missing: [] };
  for (const k of SPEC_FIELD_KEYS) {
    // Range maxima arrive with their lower value as value_max.
    if (rangeMaxKeys.has(k)) continue;
    const f = fields[k.replace(/_capacity_ml$/, "_capacity_l")];
    if (!f || f.value === null || f.value === "" || !f.source_url) {
      summary.missing.push(k);
      continue;
    }
    if (!overwrite && d.values[k].trim() !== "") {
      summary.kept.push(k);
      continue;
    }
    const text =
      typeof f.value === "number"
        ? inputText(k, metric(k, f.value), nf, units)
        : f.value;
    const meta: SpecMeta = {
      is_auto: true,
      source_url: f.source_url,
      ...(f.source_name ? { source_name: f.source_name } : {}),
    };
    draft.values[k] = text;
    draft.meta[k] = meta;
    draft.auto[k] = { value: text, meta };
    const maxKey = RANGE_MAX[k];
    if (maxKey) {
      // A single value replaces a former range; a range sets both ends.
      const hi = f.value_max ?? null;
      const hiText =
        hi === null ? "" : inputText(maxKey, metric(maxKey, hi), nf, units);
      draft.values[maxKey] = hiText;
      if (hi === null) {
        draft.meta = Object.fromEntries(
          Object.entries(draft.meta).filter(([key]) => key !== maxKey),
        );
      } else {
        draft.meta[maxKey] = meta;
        draft.auto[maxKey] = { value: hiText, meta };
      }
    }
    summary.filled.push(k);
  }
  return { draft, summary };
}

/** The PUT /specs body, or field errors for numbers that do not parse. */
export function toRequestBody(
  d: SpecsDraft,
  nf: NumberFormat,
  units: SpecUnits = METRIC_SPEC_UNITS,
): { body: Record<string, unknown>; errors: Record<string, string> } {
  const body: Record<string, unknown> = {};
  const errors: Record<string, string> = {};
  const meta: Partial<Record<SpecFieldKey, SpecMeta>> = {};
  for (const k of SPEC_FIELD_KEYS) {
    const raw = d.values[k].trim();
    if (isText(k)) {
      body[k] = raw;
    } else if (raw === "") {
      body[k] = null;
    } else {
      const n = readMetricField(
        raw,
        d.stored[k] ?? { text: "", value: null },
        unitSpec(k, units),
        nf,
        isCapacity(k),
      );
      if (n === undefined) errors[k] = "validation.number";
      body[k] = n ?? null;
    }
    const m = d.meta[k];
    if (m && raw !== "") meta[k] = m;
  }
  body.meta = meta;
  // The web app always sends the motorcycle fields; errors use the server's keys.
  const moto: Record<string, unknown> = {};
  for (const k of MOTO_TEXT) moto[k] = d.moto[k].trim();
  moto.final_drive = d.moto.final_drive === "" ? null : d.moto.final_drive;
  for (const k of [...MOTO_INTEGER, ...MOTO_MM]) {
    const raw = d.moto[k].trim();
    const n = raw === "" ? null : parseNumber(raw, nf);
    if (raw !== "" && (n === null || (MOTO_INTEGER.includes(k) && !Number.isInteger(n)))) errors[`motorcycle.${k}`] = "validation.number";
    moto[k] = n;
  }
  const oil = readMetricField(d.moto.fork_oil_capacity_ml, d.forkOil, VOLUME[units.volume], nf, true);
  if (oil === undefined) errors["motorcycle.fork_oil_capacity_ml"] = "validation.number";
  moto.fork_oil_capacity_ml = oil ?? null;
  body.motorcycle = moto;
  return { body, errors };
}
