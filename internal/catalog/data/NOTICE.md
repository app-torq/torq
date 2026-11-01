# Vehicle catalog data: sources, licenses and transformations

`catalog_seed.json.gz` in this directory is a database derived from the sources below.
`cmd/catalog-sync` generates it (see `make catalog`). An admin-triggered online update in a running torq
(Settings → About & data sources) builds the same database from freshly downloaded copies of sources 1 and 2,
with the same transformations; its retrieval date and source commit are shown on the About page.

The derived database is distributed under the **Open Database License (ODbL) 1.0**. The full text is in
[`LICENSE-ODbL.txt`](./LICENSE-ODbL.txt). The torq program code is licensed separately; the ODbL applies to
this database only.

## 1. KBA FZ 6 — Bestand an Kraftfahrzeugen und Kraftfahrzeuganhängern nach Herstellern und Typen

- **Publisher**: © Kraftfahrt-Bundesamt, Flensburg
- **Dataset**: FZ 6.1, stand 1 January 2024 (published April 2024)
- **URI**: https://www.kba.de/DE/Statistik/Fahrzeuge/Bestand/ListeHerstellerTypSchluesselnummern/typschluesselnummern_node.html
- **Retrieved**: 2026-10-31
- **License**: Datenlizenz Deutschland – Namensnennung – Version 2.0 (dl-de/by-2-0),
  https://www.govdata.de/dl-de/by-2-0. The license is stated in the workbook's "Impressum" sheet under
  "Nutzungsbedingungen".
- **Attribution**: Datenquelle: Kraftfahrt-Bundesamt, FZ 6, 2026-10-31; Datenlizenz by-2-0; Daten geändert
- **Transformations** (the data was changed):
  - Kept only manufacturers mapped to a passenger-car make in `cmd/catalog-sync/makes.go`. This drops
    trailers, tractors and commercial-only manufacturers.
  - Normalized manufacturer names to canonical makes. For example, `DAIMLERCHRYSLER (D)` became
    `Mercedes-Benz`, and `BAYER.MOT.WERKE-BMW` became `BMW`.
  - Split combined trade names (`C 220 CDI;CLC 220 CDI`) into one variant per name, and dropped purely
    numeric segments.
  - Matched a trade name to a model from source 2 only when exactly one model matches its leading word.
  - Dropped the vehicle stock counts.

## 2. gor3a/vehicle-makes-models

- **Publisher**: the vehicle-makes-models project, https://github.com/gor3a/vehicle-makes-models. The data
  was compiled from publicly available information on autoevolution.com (https://www.autoevolution.com).
- **Dataset**: `data/json/all.json` at commit `ee99776470a45214bf524cdfe22e5dbf4bd67f93` (2026-09-25)
- **Retrieved**: 2026-10-31
- **License**: Open Database License (ODbL) 1.0, https://opendatacommons.org/licenses/odbl/1-0/. This is
  the data license in `LICENSE-DATA`; the project's code is MIT.
- **Attribution**: Contains data from vehicle-makes-models (https://github.com/gor3a/vehicle-makes-models)
  and autoevolution.com, licensed under ODbL 1.0; changes were made.
- **Transformations** (the data was changed):
  - Removed make prefixes and "and predecessors" from model names.
  - Unified Mercedes-Benz model names, which the source spells in German and English ("C-Klasse T-Modell",
    "C-Class Wagon", "C-CLASS"), into one model per class ("C-Class"), moved body words ("T-Modell", "Coupe")
    into a body field and dropped the "AMG" suffix. Added German model names ("C-Klasse", BMW "3er").
  - Replaced production-year generation labels with chassis codes ("W203", "E46", "Golf IV (1J)") from the
    table `internal/catalog/catalogsync/chassis.txt` (Mercedes-Benz, BMW, Volkswagen, Audi, Porsche, Opel),
    matched by model, body and first production year. Other generations keep the remaining descriptor plus
    production years, with the descriptor translated into German ("Coupé (2016–2018)").
  - Removed the "(N HP)" suffix from engine labels.
  - Read power from the source's kW value (falling back to PS × 0.73549875) and stored it as whole kW only.
  - Rounded displacement to whole cm³.
  - Mapped fuel names to torq's fuel types.
  - Removed duplicate engines.
  - Dropped all other specifications.

## Not bundled: KBA SV 4.2

The KBA "Verzeichnis der Hersteller und Typen (SV 4.2)" carries kW, cm³ and fuel per HSN/TSN. However, the
document states "© Alle Rechte vorbehalten. Vervielfältigung und Verbreitung, auch auszugsweise, nur mit
vorheriger Zustimmung gestattet." In English: all rights reserved; copying and distribution, even of
excerpts, only with prior consent.

It is therefore **not** part of the shipped catalog (Constitution VII: only openly licensed datasets).
`cmd/catalog-sync` can read it only when the maintainer passes `--sv42-consent=yes`, which confirms that
written consent from the KBA exists. The test fixtures for that parser are synthetic and contain no KBA
content.
