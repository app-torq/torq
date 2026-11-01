package catalogsync

import "testing"

func i64(v int64) *int64 { return &v }

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		make, model, gen string
		from, to         *int64
		want             taxonomy
	}{
		// The same Mercedes model in German, English and upper case.
		{"Mercedes-Benz", "MERCEDES BENZ C-Klasse T-Modell", "MERCEDES BENZ C-Klasse T-Modell (2004)", i64(2004), i64(2007),
			taxonomy{names{"C-Class", "C-Klasse"}, same("S203"), "estate"}},
		{"Mercedes-AMG", "C-Class", "C-Class Wagon (2015)", i64(2015), i64(2018),
			taxonomy{names{"C-Class", "C-Klasse"}, same("S205"), "estate"}},
		{"Mercedes-AMG", "C-CLASS", "C-CLASS (2015)", i64(2015), i64(2018),
			taxonomy{names{"C-Class", "C-Klasse"}, same("W205"), ""}},
		{"Mercedes-Benz", "MERCEDES BENZ C-Klasse SportCoupe/CLC", "MERCEDES BENZ C-Klasse SportCoupe/CLC (2008)", i64(2008), i64(2011),
			taxonomy{names{"C-Class", "C-Klasse"}, same("CL203"), "coupe"}},
		{"Mercedes-Benz", "MERCEDES BENZ E-Class T-Modell All-Terrain", "MERCEDES BENZ E-Class T-Modell All-Terrain (2017)", i64(2017), i64(2020),
			taxonomy{names{"E-Class All-Terrain", "E-Klasse All-Terrain"}, same("S213"), "estate"}},
		{"Mercedes-Benz", "MERCEDES BENZ GLS- Class", "MERCEDES BENZ GLS- Class (2019)", i64(2019), i64(2023),
			taxonomy{same("GLS"), same("X167"), ""}},
		{"Mercedes-Benz", "MERCEDES BENZ CLS Shooting Brake AMG", "MERCEDES BENZ CLS Shooting Brake AMG (2014)", i64(2014), i64(2018),
			taxonomy{same("CLS Shooting Brake"), same("X218"), "estate"}},
		// Unknown words without a class word keep the source name.
		{"Mercedes-Benz", "MERCEDES BENZ CLK GTR", "MERCEDES BENZ CLK GTR (1998)", i64(1998), i64(1999),
			taxonomy{same("CLK GTR"), same("1998–1999"), ""}},
		{"Mercedes-Benz", "MERCEDES BENZ VIANO", "MERCEDES BENZ VIANO (2010)", i64(2010), i64(2014),
			taxonomy{same("Viano"), same("W639"), ""}},
		// No chassis code: localized descriptor plus years.
		{"Mercedes-Benz", "MERCEDES BENZ E-Klasse T-Modell", "MERCEDES BENZ E-Klasse T-Modell (1965)", i64(1965), i64(1968),
			taxonomy{names{"E-Class", "E-Klasse"}, names{"Estate (1965–1968)", "T-Modell (1965–1968)"}, ""}},
		{"Mercedes-Benz", "MERCEDES BENZ S-Class", "MERCEDES BENZ S-Class Cabriolet (1952)", i64(1952), i64(1958),
			taxonomy{names{"S-Class", "S-Klasse"}, names{"Convertible (1952–1958)", "Cabriolet (1952–1958)"}, ""}},
		// BMW: German series names, body-specific codes, split "Gran Coupe".
		{"BMW", "3 Series", "3 Series Touring (2012)", i64(2012), i64(2016),
			taxonomy{names{"3 Series", "3er"}, same("F31"), "estate"}},
		{"BMW", "3 Series", "3 Series Sedan (2018)", i64(2018), i64(2022),
			taxonomy{names{"3 Series", "3er"}, same("G20"), "sedan"}},
		{"BMW", "4 Series Gran", "4 Series Gran Coupe (2021)", i64(2021), i64(2024),
			taxonomy{names{"4 Series Gran Coupe", "4er Gran Coupé"}, same("G26"), "coupe"}},
		{"BMW", "M3 CS", "M3 CS Touring (2025)", i64(2025), nil,
			taxonomy{same("M3 CS"), same("G81"), "estate"}},
		// Body words in model names steer the lookup.
		{"Volkswagen", "Golf R Cabrio", "Golf R Cabrio (2013)", i64(2013), i64(2017),
			taxonomy{same("Golf R Cabrio"), same("Golf VI (1K)"), "cabriolet"}},
		{"Volkswagen", "Golf GTD Variant", "Golf GTD Variant (2015)", i64(2015), i64(2021),
			taxonomy{same("Golf GTD Variant"), same("Golf VII (BA5)"), "estate"}},
		{"Audi", "RS 5", "RS 5 RS5 Sportback (2019)", i64(2019), nil,
			taxonomy{same("RS 5"), same("B9"), "sportback"}},
		{"Porsche", "911 Carrera 4S", "911 Carrera 4S Cabriolet (2012)", i64(2012), i64(2018),
			taxonomy{same("911 Carrera 4S"), same("991"), "cabriolet"}},
		// A code ends with its production; later generations fall back.
		{"Porsche", "Cayenne Electric", "Cayenne Electric (2026)", i64(2026), nil,
			taxonomy{same("Cayenne Electric"), same("2026–"), ""}},
		{"Opel", "Astra", "Astra 5 Doors (2007)", i64(2007), i64(2009),
			taxonomy{same("Astra"), same("Astra H"), "hatchback"}},
		{"Opel", "Astra", "Astra 3 Doors (2026)", i64(2026), nil,
			taxonomy{same("Astra"), same("Astra L"), "hatchback"}},
		{"Toyota", "Corolla", "Corolla Sedan (2019)", i64(2019), nil,
			taxonomy{same("Corolla"), names{"Sedan (2019–)", "Limousine (2019–)"}, ""}},
	} {
		got := classify(c.make, c.model, c.gen, c.from, c.to, newReport())
		if got != c.want {
			t.Errorf("%s %q / %q:\n got %+v\nwant %+v", c.make, c.model, c.gen, got, c.want)
		}
	}
}

func TestChassisTableRejectsBadLines(t *testing.T) {
	for name, line := range map[string]string{
		"fields":    "BMW | 3 Series | E46 1998-2006",
		"body":      "BMW | 3 Series | wagon | E46 1998-2006",
		"years":     "BMW | 3 Series | | E46 2006-1998",
		"format":    "BMW | 3 Series | | E46",
		"duplicate": "BMW | 3 Series | | E46 1998-2006\nBMW | 3 Series | | E90 2005-2012",
	} {
		if _, err := parseChassis(line); err == nil {
			t.Errorf("%s: accepted %q", name, line)
		}
	}
}
