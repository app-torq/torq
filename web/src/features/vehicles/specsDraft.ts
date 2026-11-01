// Editable state of the "Edit data" spec dialog: the input texts plus, per
// field, where the value came from (web auto-fill or a manual adjustment).
// Pure functions so the rules are easy to test; nothing is saved until the
// user saves the dialog.
import { parseNumber, toInputValue } from "../../lib/format";
import {
  SPEC_FIELD_KEYS,
  type AutofillValue,
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
}

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
const decimals = (k: SpecFieldKey) =>
  k === "wheel_lug_torque_nm" ? 1 : k.startsWith("tire_pressure") ? 2 : 3;

/** Capacities are stored in mL but entered in litres. */
function inputText(
  k: SpecFieldKey,
  n: number | null,
  nf: NumberFormat,
): string {
  return toInputValue(n, nf, decimals(k));
}

export function initialDraft(
  specs: VehicleSpecs,
  nf: NumberFormat,
): SpecsDraft {
  const values = {} as Record<SpecFieldKey, string>;
  const auto: SpecsDraft["auto"] = {};
  for (const k of SPEC_FIELD_KEYS) {
    const v = specs[k];
    values[k] =
      typeof v === "string"
        ? v
        : inputText(k, v === null ? null : isCapacity(k) ? v / 1000 : v, nf);
    const m = specs.meta[k];
    if (m?.is_auto && values[k] !== "") auto[k] = { value: values[k], meta: m };
  }
  return { values, meta: { ...specs.meta }, auto };
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
): { draft: SpecsDraft; summary: AutofillSummary } {
  const draft: SpecsDraft = {
    values: { ...d.values },
    meta: { ...d.meta },
    auto: { ...d.auto },
  };
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
      typeof f.value === "number" ? inputText(k, f.value, nf) : f.value;
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
      const hiText = hi === null ? "" : inputText(maxKey, hi, nf);
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
      const n = parseNumber(raw, nf);
      if (n === null) errors[k] = "validation.number";
      body[k] = n === null ? null : isCapacity(k) ? Math.round(n * 1000) : n;
    }
    const m = d.meta[k];
    if (m && raw !== "") meta[k] = m;
  }
  body.meta = meta;
  return { body, errors };
}
