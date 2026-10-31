package catalogsync

import (
	"regexp"
	"strings"
)

// manufacturers maps KBA "Herstellerklartext" spellings to canonical makes
// (research C4). Only passenger-car manufacturers are listed; everything else
// (trailers, tractors, trucks, motorcycle-only makers) is skipped and the
// largest skipped names appear in the sync report for review.
var manufacturers = map[string]string{
	"VOLKSWAGEN-VW":          "Volkswagen",
	"BAYER.MOT.WERKE-BMW":    "BMW",
	"BMW M":                  "BMW",
	"BMW MOTORSPORT":         "BMW",
	"ALPINA":                 "Alpina",
	"AUDI":                   "Audi",
	"AUDI (H)":               "Audi",
	"AUDI SPORT":             "Audi",
	"QUATTRO":                "Audi",
	"AUDI NSU AUTO UNION":    "Audi",
	"NSU-MOTORENWERKE":       "NSU",
	"AUTO UNION":             "Auto Union",
	"DAIMLER (D)":            "Mercedes-Benz",
	"DAIMLERCHRYSLER (D)":    "Mercedes-Benz",
	"DAIMLER-BENZ":           "Mercedes-Benz",
	"MERCEDES-BENZ":          "Mercedes-Benz",
	"MERCEDES-BENZ (E)":      "Mercedes-Benz",
	"MERCEDES-AMG":           "Mercedes-AMG",
	"SMART":                  "Smart",
	"MCC SMART":              "Smart",
	"MCC (CH)":               "Smart",
	"SMART (RC)":             "Smart",
	"PORSCHE":                "Porsche",
	"OPEL":                   "Opel",
	"OPEL AUTOMOBILE":        "Opel",
	"GENERAL MOTORS (E)":     "Opel",
	"SOVAB (F)":              "Opel",
	"VAUXHALL (UK)":          "Vauxhall",
	"FORD (D)":               "Ford",
	"FORD (USA)":             "Ford",
	"FORD (AUS)":             "Ford",
	"FORD (IND)":             "Ford",
	"FORD (B)":               "Ford",
	"FORD (E)":               "Ford",
	"FORD (NL)":              "Ford",
	"FORD (TR)":              "Ford",
	"FORD (UK)":              "Ford",
	"SKODA (CZ)":             "Škoda",
	"AZNP-SKODA (CS)":        "Škoda",
	"SEAT (E)":               "SEAT",
	"RENAULT (F)":            "Renault",
	"DACIA (RO)":             "Dacia",
	"ALPINE (F)":             "Alpine",
	"PEUGEOT (F)":            "Peugeot",
	"CITROEN (F)":            "Citroën",
	"PSA AUTOMOBILES (F)":    "Peugeot",
	"FIAT (I)":               "Fiat",
	"FCA ITALY (I)":          "Fiat",
	"ALFA ROMEO (I)":         "Alfa Romeo",
	"ALFA LANCIA (I)":        "Lancia",
	"FIAT LANCIA (I)":        "Lancia",
	"LANCIA (I)":             "Lancia",
	"FERRARI (I)":            "Ferrari",
	"MASERATI (I)":           "Maserati",
	"LAMBORGHINI (I)":        "Lamborghini",
	"PININFARINA (I)":        "Pininfarina",
	"BUGATTI (F)":            "Bugatti",
	"MATRA (F)":              "Matra",
	"VOLVO (S)":              "Volvo",
	"VOLVO (B)":              "Volvo",
	"VOLVO (NL)":             "Volvo",
	"POLESTAR (S)":           "Polestar",
	"SAAB AUTOMOBILE (S)":    "Saab",
	"SAAB-SCANIA (S)":        "Saab",
	"SAAB (S)":               "Saab",
	"SAAB EUROPE (B)":        "Saab",
	"SAAB VALMET (SF)":       "Saab",
	"JAGUAR LAND ROVER (UK)": "Jaguar",
	"JAGUAR (UK)":            "Jaguar",
	"LAND ROVER (UK)":        "Land Rover",
	"LAND ROVER GROUP(UK)":   "Land Rover",
	"ROVER (UK)":             "Rover",
	"MG ROVER (UK)":          "Rover",
	"AUSTIN ROVER (UK)":      "Rover",
	"BENTLEY (UK)":           "Bentley",
	"ROLLS-ROYCE (UK)":       "Rolls-Royce",
	"ROLLS-ROYCE MC (UK)":    "Rolls-Royce",
	"MCLAREN (UK)":           "McLaren",
	"WUHAN LOTUS (RC)":       "Lotus",
	"INEOS AUTOMOTIVE (UK)":  "Ineos",
	"TOYOTA EUROPE (B)":      "Toyota",
	"TOYOTA MEM (B)":         "Toyota",
	"TOYOTA (J)":             "Toyota",
	"TOYOTA (ZA)":            "Toyota",
	"TOYOTA (UK)":            "Toyota",
	"TOYOTA (USA)":           "Toyota",
	"TOYOTA (THA)":           "Toyota",
	"TOYOTA CAETANO (P)":     "Toyota",
	"HONDA MOTOR (J)":        "Honda",
	"HONDA (UK)":             "Honda",
	"HONDA (USA)":            "Honda",
	"HONDA TURKIYE (TR)":     "Honda",
	"NISSAN (CH)":            "Nissan",
	"NISSAN EUROPE (F)":      "Nissan",
	"NISSAN AUTOMOTIVE (F)":  "Nissan",
	"NISSAN EUROPE (NL)":     "Nissan",
	"NISSAN (J)":             "Nissan",
	"NISSAN (E)":             "Nissan",
	"NISSAN (UK)":            "Nissan",
	"NISSAN VEHICULOS (E)":   "Nissan",
	"MAZDA (J)":              "Mazda",
	"MAZDA (B)":              "Mazda",
	"MAZDA (USA)":            "Mazda",
	"TOYO KOGYO (J)":         "Mazda",
	"MITSUBISHI (J)":         "Mitsubishi",
	"MITSUBISHI (THA)":       "Mitsubishi",
	"MITSUBISHI (NL)":        "Mitsubishi",
	"MITSUBISHI (AUS)":       "Mitsubishi",
	"MITSUBISHI (USA)":       "Mitsubishi",
	"NETHERLANDS CAR (NL)":   "Mitsubishi",
	"SUZUKI (J)":             "Suzuki",
	"MAGYAR SUZUKI (H)":      "Suzuki",
	"MARUTI (IND)":           "Suzuki",
	"SUBARU (J)":             "Subaru",
	"FUJI HEAVY IND.(J)":     "Subaru",
	"DAIHATSU (J)":           "Daihatsu",
	"ISUZU (J)":              "Isuzu",
	"HYUNDAI MOTOR (CZ)":     "Hyundai",
	"HYUNDAI (TR)":           "Hyundai",
	"HYUNDAI MOTOR (ROK)":    "Hyundai",
	"HYUNDAI MOTOR (IND)":    "Hyundai",
	"KIA MOTORS (SK)":        "Kia",
	"KIA MOTOR (ROK)":        "Kia",
	"KIA (SK)":               "Kia",
	"KIA (ROK)":              "Kia",
	"SSANGYONG (ROK)":        "SsangYong",
	"DAEWOO (ROK)":           "Daewoo",
	"GM DAEWOO (ROK)":        "Chevrolet",
	"GM KOREA":               "Chevrolet",
	"GENERAL MOT-GMC(USA)":   "Chevrolet",
	"GENERAL MOTORS (USA)":   "Chevrolet",
	"CHRYSLER (USA)":         "Chrysler",
	"FCA (USA)":              "Jeep",
	"JEEP EAGLE (USA)":       "Jeep",
	"TESLA (USA)":            "Tesla",
	"LUCID (USA)":            "Lucid",
	"RIVIAN (USA)":           "Rivian",
	"FISKER (USA)":           "Fisker",
	"FISKER GROUP (USA)":     "Fisker",
	"SAIC (RC)":              "MG",
	"ZHEJIANG GEELY (RC)":    "Lynk & Co",
	"GREAT WALL MOTOR (RC)":  "Ora",
	"BYD (RC)":               "BYD",
	"NIO (RC)":               "NIO",
	"LADA":                   "Lada",
	"AVTOVAZ (RUS)":          "Lada",
	"WOLGA-AUTOW.-VAZ/RUS":   "Lada",
	"AUTOMOB-EISENACH-AWE":   "Wartburg",
	"SACHSENRING":            "Trabant",
	"TATA ENG.(IND)":         "Tata",
	"SANTANA (E)":            "Santana",
	"GLAS":                   "Glas",
}

// brandWords re-assign a trade-name variant to another brand of the same
// manufacturer group when the variant starts with one of these words, e.g.
// "LEXUS IS 200" from Toyota or "COOPER" from BMW (research C4). Keys are the
// first word, or the first two words joined by a space.
var brandWords = map[string]string{
	"LEXUS": "Lexus", "DAIHATSU": "Daihatsu", "DATSUN": "Datsun",
	"MINI": "MINI", "COOPER": "MINI", "ONE": "MINI",
	"CUPRA": "Cupra", "BORN": "Cupra", "FORMENTOR": "Cupra", "TAVASCAN": "Cupra",
	"ABARTH": "Abarth", "ALFA": "Alfa Romeo", "LANCIA": "Lancia", "FIAT": "Fiat",
	"JEEP": "Jeep", "WRANGLER": "Jeep", "CHEROKEE": "Jeep", "GRAND CHEROKEE": "Jeep", "COMPASS": "Jeep",
	"PATRIOT": "Jeep", "COMMANDER": "Jeep", "RENEGADE": "Jeep", "GLADIATOR": "Jeep", "AVENGER": "Jeep",
	"CALIBER": "Dodge", "NITRO": "Dodge", "JOURNEY": "Dodge", "DODGE": "Dodge",
	"CHRYSLER": "Chrysler", "CHEVROLET": "Chevrolet", "DAEWOO": "Daewoo",
	"CADILLAC": "Cadillac", "ESCALADE": "Cadillac", "CTS": "Cadillac", "CTS-V": "Cadillac", "SRX": "Cadillac",
	"STS": "Cadillac", "ATS": "Cadillac", "ATS-V": "Cadillac", "XT4": "Cadillac", "XT5": "Cadillac", "CT6": "Cadillac",
	"HUMMER": "Hummer", "PONTIAC": "Pontiac",
	"JAGUAR": "Jaguar", "RANGE": "Land Rover", "DISCOVERY": "Land Rover", "DEFENDER": "Land Rover",
	"FREELANDER": "Land Rover", "EVOQUE": "Land Rover",
	"LYNK": "Lynk & Co", "ORA": "Ora", "WEY": "WEY", "MG": "MG",
	"DS": "DS", "DS3": "DS", "DS4": "DS", "DS5": "DS", "DS7": "DS", "DS9": "DS",
}

// groupModelBrands assign variants of multi-brand manufacturers by model word.
var groupModelBrands = map[string]map[string]string{
	"PSA AUTOMOBILES (F)": {
		"CORSA": "Opel", "MOKKA": "Opel", "ASTRA": "Opel", "GRANDLAND": "Opel", "CROSSLAND": "Opel", "COMBO": "Opel",
		"ZAFIRA": "Opel", "VIVARO": "Opel", "FRONTERA": "Opel",
		"BERLINGO": "Citroën", "C3": "Citroën", "C4": "Citroën", "C5": "Citroën", "E-C4": "Citroën", "E-BERLINGO": "Citroën",
		"SPACETOURER": "Citroën", "JUMPY": "Citroën", "AMI": "Citroën",
	},
}

var wordRe = regexp.MustCompile(`[A-Z0-9][A-Z0-9-]*`)

// brandFor returns the make for one trade-name variant of a manufacturer.
func brandFor(manufacturer, base, tradeName string) string {
	words := wordRe.FindAllString(strings.ToUpper(tradeName), 2)
	if len(words) == 0 {
		return base
	}
	if len(words) == 2 {
		if b, ok := brandWords[words[0]+" "+words[1]]; ok && sellsBrand(base, b) {
			return b
		}
	}
	if b, ok := groupModelBrands[manufacturer][words[0]]; ok {
		return b
	}
	if b, ok := brandWords[words[0]]; ok && sellsBrand(base, b) {
		return b
	}
	return base
}

// groupBrands lists the brands each base make's manufacturers also sell, so
// "ONE" from Honda or "MG" from another maker is never re-labelled.
var groupBrands = map[string][]string{
	"Toyota":    {"Lexus", "Daihatsu"},
	"Nissan":    {"Datsun"},
	"BMW":       {"MINI"},
	"SEAT":      {"Cupra"},
	"Fiat":      {"Abarth", "Alfa Romeo", "Lancia", "Jeep"},
	"Lancia":    {"Alfa Romeo"},
	"Chrysler":  {"Jeep", "Dodge"},
	"Jeep":      {"Chrysler", "Dodge"},
	"Chevrolet": {"Daewoo", "Cadillac", "Hummer", "Pontiac"},
	"Jaguar":    {"Land Rover"},
	"Peugeot":   {"DS", "Opel", "Citroën"},
	"Citroën":   {"DS"},
	"Ora":       {"WEY"},
}

func sellsBrand(base, brand string) bool {
	if brand == base {
		return true
	}
	for _, b := range groupBrands[base] {
		if b == brand {
			return true
		}
	}
	return false
}
