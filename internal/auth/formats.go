package auth

import (
	"context"
	"slices"
	"strings"

	"github.com/app-torq/torq/internal/apperr"
	"github.com/app-torq/torq/internal/store"
)

// formatChoices lists the valid values of each format preference, keyed by
// its API field name.
var formatChoices = map[string][]string{
	"date_format":       {"DD.MM.YYYY", "YYYY-MM-DD", "MM/DD/YYYY"},
	"number_format":     {"point", "comma"},
	"time_format":       {"24h", "12h"},
	"power_unit":        {"kw", "ps", "hp"},
	"distance_unit":     {"km", "mi"},
	"volume_unit":       {"l", "qt"},
	"torque_unit":       {"nm", "lbft"},
	"pressure_unit":     {"bar", "psi", "kpa"},
	"displacement_unit": {"cc", "l", "ci"},
}

// Validate normalizes the currency code and reports invalid choices. Field
// names get prefix, e.g. "default_" for the instance defaults.
func (f *Formats) Validate(v *apperr.Validation, prefix string) {
	f.CurrencyCode = strings.ToUpper(strings.TrimSpace(f.CurrencyCode))
	v.Check(!currencyCodeRe.MatchString(f.CurrencyCode), prefix+"currency_code", "validation.currency_code")
	for field, value := range map[string]string{
		"date_format": f.DateFormat, "number_format": f.NumberFormat, "time_format": f.TimeFormat,
		"power_unit": f.PowerUnit, "distance_unit": f.DistanceUnit, "volume_unit": f.VolumeUnit,
		"torque_unit": f.TorqueUnit, "pressure_unit": f.PressureUnit, "displacement_unit": f.DisplacementUnit,
	} {
		v.Check(!slices.Contains(formatChoices[field], value), prefix+field, "validation.invalid_choice")
	}
}

// DefaultsFromStore returns the formats new users start with.
func DefaultsFromStore(c store.InstanceSetting) Formats {
	return Formats{
		DateFormat: c.DefaultDateFormat, NumberFormat: c.DefaultNumberFormat, TimeFormat: c.DefaultTimeFormat,
		CurrencyCode: c.DefaultCurrencyCode, PowerUnit: c.DefaultPowerUnit, DistanceUnit: c.DefaultDistanceUnit,
		VolumeUnit: c.DefaultVolumeUnit, TorqueUnit: c.DefaultTorqueUnit, PressureUnit: c.DefaultPressureUnit,
		DisplacementUnit: c.DefaultDisplacementUnit,
	}
}

// SaveDefaults validates f and stores it as the instance defaults.
func SaveDefaults(ctx context.Context, q *store.Queries, f Formats) (store.InstanceSetting, error) {
	var v apperr.Validation
	f.Validate(&v, "default_")
	if err := v.Err(); err != nil {
		return store.InstanceSetting{}, err
	}
	return q.UpdateInstanceDefaults(ctx, store.UpdateInstanceDefaultsParams{
		DefaultDateFormat: f.DateFormat, DefaultNumberFormat: f.NumberFormat, DefaultTimeFormat: f.TimeFormat,
		DefaultCurrencyCode: f.CurrencyCode, DefaultPowerUnit: f.PowerUnit, DefaultDistanceUnit: f.DistanceUnit,
		DefaultVolumeUnit: f.VolumeUnit, DefaultTorqueUnit: f.TorqueUnit, DefaultPressureUnit: f.PressureUnit,
		DefaultDisplacementUnit: f.DisplacementUnit,
	})
}
