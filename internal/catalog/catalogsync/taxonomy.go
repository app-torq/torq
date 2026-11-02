package catalogsync

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/app-torq/torq/internal/catalog"
)

// names is a display name in both catalog languages.
type names struct{ en, de string }

func same(s string) names { return names{s, s} }

// deOnly returns the German name when it differs; the seed omits identical
// names.
func deOnly(n names) string {
	if n.de == n.en {
		return ""
	}
	return n.de
}

// taxonomy is the cleaned-up model and generation of one source generation.
type taxonomy struct {
	model, generation names
	body              string // catalog.Bodies token; set only with a chassis code
}

// classify derives the localized model, the generation and the body of one
// source generation. The generation is a chassis code when chassis.txt knows
// one ("W203", "E46"); otherwise it is the source's descriptor plus the
// production years ("Coupé (2016–2018)").
func classify(make, rawModel, genName string, from, to *int64, rep *Report) taxonomy {
	model := cleanModel(make, rawModel)
	desc := descriptor(make, rawModel, model, genName)
	model, desc = mergeCoupe(model, desc)
	m := localizeModel(make, model)
	descBody, descLabel, known := describe(desc)
	body := cmp.Or(descBody, m.body)
	t := taxonomy{model: m.names}
	if code, ok := chassis.lookup(make, m.names.en, body, from); ok {
		t.generation, t.body = same(code), body
		rep.chassis(make, true)
		return t
	}
	rep.chassis(make, false)
	if !known {
		descLabel = same(desc)
	}
	if desc == "" && m.bodyLabel != nil {
		descLabel = *m.bodyLabel
	}
	years := yearRange(from, to)
	t.generation = names{withYears(descLabel.en, years), withYears(descLabel.de, years)}
	return t
}

// descriptor is the part of a generation name after the model, without the
// "(2016)" year suffix: "MERCEDES BENZ C-Klasse Coupe (2016)" → "Coupe".
func descriptor(make, rawModel, model, name string) string {
	desc := strings.TrimSpace(name)
	if len(desc) >= len(rawModel) && strings.EqualFold(desc[:len(rawModel)], rawModel) {
		desc = desc[len(rawModel):]
	} else {
		desc = cleanModel(make, desc)
		if len(desc) >= len(model) && strings.EqualFold(desc[:len(model)], model) {
			desc = desc[len(model):]
		}
	}
	desc = strings.TrimSpace(yearSuffixRe.ReplaceAllString(desc, ""))
	// "RS5 Sportback" under the model "RS 5": drop the repeated model.
	if w := strings.Fields(desc); len(w) > 1 && catalog.Key(w[0]) == catalog.Key(model) {
		desc = strings.Join(w[1:], " ")
	}
	return desc
}

// mergeCoupe moves "Coupe" back into model names the source split between
// model and generation ("4 Series Gran" + "Coupe", "GT 4-DOOR" + "COUPE").
func mergeCoupe(model, desc string) (string, string) {
	w := strings.Fields(desc)
	lm := strings.ToLower(model)
	if len(w) > 0 && strings.EqualFold(w[0], "coupe") && (strings.HasSuffix(lm, " gran") || strings.HasSuffix(lm, " 4-door")) {
		return model + " Coupe", strings.Join(w[1:], " ")
	}
	return model, desc
}

func yearRange(from, to *int64) string {
	switch {
	case from != nil && to != nil && *to != *from:
		return fmt.Sprintf("%d–%d", *from, *to)
	case from != nil && to != nil:
		return strconv.FormatInt(*from, 10)
	case from != nil:
		return fmt.Sprintf("%d–", *from)
	}
	return ""
}

func withYears(desc, years string) string {
	switch {
	case desc == "":
		return years
	case years == "":
		return desc
	default:
		return desc + " (" + years + ")"
	}
}

// bodyPhrase maps a body word of the source to a body token and its label.
type bodyPhrase struct {
	body  string
	label names
}

// bodyPhrases are matched case-insensitively against a whole generation
// descriptor and against words of model names.
var bodyPhrases = map[string]bodyPhrase{
	"wagon":         {"estate", names{"Estate", "Kombi"}},
	"estate":        {"estate", names{"Estate", "Kombi"}},
	"t-modell":      {"estate", names{"Estate", "T-Modell"}},
	"touring":       {"estate", same("Touring")},
	"avant":         {"estate", same("Avant")},
	"variant":       {"estate", same("Variant")},
	"caravan":       {"estate", same("Caravan")},
	"sports tourer": {"estate", same("Sports Tourer")},
	"coupe":         {"coupe", names{"Coupe", "Coupé"}},
	"coupé":         {"coupe", names{"Coupe", "Coupé"}},
	"sportcoupe":    {"coupe", names{"Sport Coupe", "Sportcoupé"}},
	"cabriolet":     {"cabriolet", names{"Convertible", "Cabriolet"}},
	"cabrio":        {"cabriolet", names{"Convertible", "Cabrio"}},
	"convertible":   {"cabriolet", names{"Convertible", "Cabrio"}},
	"roadster":      {"roadster", same("Roadster")},
	"spyder":        {"roadster", same("Spyder")},
	"sedan":         {"sedan", names{"Sedan", "Limousine"}},
	"saloon":        {"sedan", names{"Sedan", "Limousine"}},
	"hatchback":     {"hatchback", names{"Hatchback", "Schrägheck"}},
	"3 doors":       {"hatchback", names{"3-door", "3-Türer"}},
	"5 doors":       {"hatchback", names{"5-door", "5-Türer"}},
	"sportback":     {"sportback", same("Sportback")},
}

// describe recognizes a descriptor that only names a body style.
func describe(desc string) (body string, label names, known bool) {
	if p, ok := bodyPhrases[strings.ToLower(strings.Join(strings.Fields(desc), " "))]; ok {
		return p.body, p.label, true
	}
	return "", names{}, desc == ""
}

// modelBody finds a body word inside a model name ("Golf Variant",
// "M6 Cabrio", "Insignia Sports Tourer"); it only steers the chassis lookup.
func modelBody(model string) string {
	w := strings.Fields(strings.ToLower(model))
	for i := range w {
		if i+1 < len(w) {
			if p, ok := bodyPhrases[w[i]+" "+w[i+1]]; ok {
				return p.body
			}
		}
		if p, ok := bodyPhrases[w[i]]; ok {
			return p.body
		}
	}
	return ""
}

// modelInfo is a localized model name. body is a body style found in the
// model; bodyLabel is set when that body word was removed from the name.
type modelInfo struct {
	names     names
	body      string
	bodyLabel *names
}

func localizeModel(make, model string) modelInfo {
	switch make {
	case "Mercedes-Benz", "Mercedes-AMG":
		return mercedesModel(model)
	case "BMW":
		return modelInfo{names: bmwModel(model), body: modelBody(model)}
	}
	return modelInfo{names: same(model), body: modelBody(model)}
}

var (
	// mercedesNameRe splits "C-Klasse T-Modell", "C-CLASS", "GLS- Class" and
	// "CLK AMG Cabrio" into class letters, the class word and the rest.
	// Longer classes come first; a class must end at a separator.
	mercedesNameRe = regexp.MustCompile(`(?i)^(CLA|CLC|CLE|CLK|CLS|GLA|GLB|GLC|GLE|GLK|GLS|SLC|SLK|CL|GL|SL|A|B|C|E|G|M|R|S|T|V|X)(?:[- ]*(Class|Klasse)\b)?(?:$|[- ]+(.*)$)`)
	// mercedesBareClasses are named without "-Class" since Mercedes' 2015
	// naming scheme (GLC, CLA, …).
	mercedesBareClasses = map[string]bool{"CLA": true, "CLE": true, "GLA": true, "GLB": true, "GLC": true, "GLE": true, "GLS": true, "SLC": true}
	mercedesRenames     = map[string]names{
		"gt4doorcoupe": {"AMG GT 4-Door Coupe", "AMG GT 4-Türer Coupé"},
	}
	allCapsRe = regexp.MustCompile(`^[A-Z]{4,}$`)
)

// mercedesSuffixes stay in the model name; the source mixes German and
// English spellings of them.
var mercedesSuffixes = map[string]struct {
	label names
	body  string
}{
	"all-terrain": {same("All-Terrain"), "estate"},
	"kurz":        {names{"Short", "kurz"}, ""},
	"lang":        {names{"Long", "lang"}, ""},
	"maybach":     {same("Maybach"), ""},
}

// mercedesModel names a Mercedes model by class ("C-Class" / "C-Klasse").
// Body words ("T-Modell", "Wagon", "Coupe") move out of the name into the
// body, and "AMG" is dropped because AMG models share their chassis codes.
// Names with words it does not know ("CLK GTR", "G 63") stay as they are
// unless the source spelled out the class word.
func mercedesModel(model string) modelInfo {
	if n, ok := mercedesRenames[catalog.Key(model)]; ok {
		return modelInfo{names: n}
	}
	m := mercedesNameRe.FindStringSubmatch(model)
	if m == nil {
		if allCapsRe.MatchString(model) {
			model = model[:1] + strings.ToLower(model[1:])
		}
		return modelInfo{names: same(model)}
	}
	class, hasClassWord := strings.ToUpper(m[1]), m[2] != ""
	var info modelInfo
	var suffix names
	shootingBrake, unknown := false, false
	words := strings.Fields(m[3])
	for i := 0; i < len(words); i++ {
		w := strings.ToLower(words[i])
		if i+1 < len(words) && w == "shooting" && strings.EqualFold(words[i+1], "brake") {
			shootingBrake, info.body = true, "estate"
			i++
			continue
		}
		if w == "amg" {
			continue
		}
		if w == "sportcoupe/clc" {
			w = "sportcoupe"
		}
		if p, ok := bodyPhrases[w]; ok {
			info.body, info.bodyLabel = p.body, &p.label
			continue
		}
		if s, ok := mercedesSuffixes[w]; ok {
			suffix = names{suffix.en + " " + s.label.en, suffix.de + " " + s.label.de}
			info.body = cmp.Or(s.body, info.body)
			continue
		}
		unknown = true
		suffix = names{suffix.en + " " + words[i], suffix.de + " " + words[i]}
	}
	if unknown && !hasClassWord {
		return modelInfo{names: same(model)}
	}
	switch {
	case shootingBrake:
		info.names = same(class + " Shooting Brake")
	case mercedesBareClasses[class]:
		info.names = same(class)
	default:
		info.names = names{class + "-Class", class + "-Klasse"}
	}
	info.names = names{info.names.en + suffix.en, info.names.de + suffix.de}
	return info
}

var bmwSeriesNameRe = regexp.MustCompile(`^([1-8]) Series\b(.*)$`)

// bmwModel names BMW series the German way ("3er", "4er Gran Coupé").
func bmwModel(model string) names {
	m := bmwSeriesNameRe.FindStringSubmatch(model)
	if m == nil {
		return same(model)
	}
	return names{model, m[1] + "er" + strings.Replace(m[2], "Gran Coupe", "Gran Coupé", 1)}
}
