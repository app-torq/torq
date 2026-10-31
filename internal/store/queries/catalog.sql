-- Offline vehicle catalog (specs/002-vehicle-catalog-lookup). Search arguments
-- are pre-normalized with catalog.Key; *_key columns are written by the seeder.

-- name: LookupKBA :many
SELECT * FROM catalog_kba
WHERE hsn = ? AND tsn = ?
ORDER BY trade_name, year_from, power_kw, id;

-- name: CatalogMakes :many
SELECT CAST(MIN(make) AS TEXT) AS name
FROM catalog_vehicles, (SELECT CAST(sqlc.arg(q) AS TEXT) AS q) AS p
WHERE instr(make_key, p.q) > 0
GROUP BY make_key
ORDER BY substr(make_key, 1, length(p.q)) <> p.q, name COLLATE NOCASE
LIMIT 25;

-- name: CatalogModels :many
-- Names are returned in the requested language (lang 'de' or 'en'); queries
-- and parents match either language.
SELECT CAST(MIN(CASE WHEN p.lang = 'de' THEN model_de ELSE model END) AS TEXT) AS name,
       CAST(COALESCE(MIN(year_from), 0) AS INTEGER) AS year_from,
       CAST(CASE WHEN SUM(year_to IS NULL) > 0 THEN 0 ELSE MAX(year_to) END AS INTEGER) AS year_to
FROM catalog_vehicles, (SELECT CAST(sqlc.arg(q) AS TEXT) AS q, CAST(sqlc.arg(lang) AS TEXT) AS lang) AS p
WHERE make_key = sqlc.arg(make_key) AND (instr(model_key, p.q) > 0 OR instr(model_de_key, p.q) > 0)
GROUP BY model_key
ORDER BY substr(model_key, 1, length(p.q)) <> p.q AND substr(model_de_key, 1, length(p.q)) <> p.q,
         name COLLATE NOCASE
LIMIT 25;

-- name: CatalogGenerations :many
-- body is a token from catalog.Bodies (or empty when the generation spans
-- several bodies); the frontend translates it.
SELECT CAST(MIN(CASE WHEN p.lang = 'de' THEN generation_de ELSE generation END) AS TEXT) AS name,
       CAST(COALESCE(MIN(year_from), 0) AS INTEGER) AS year_from,
       CAST(CASE WHEN SUM(year_to IS NULL) > 0 THEN 0 ELSE MAX(year_to) END AS INTEGER) AS year_to,
       CAST(CASE WHEN COUNT(DISTINCT body) = 1 THEN MIN(body) ELSE '' END AS TEXT) AS body
FROM catalog_vehicles, (SELECT CAST(sqlc.arg(q) AS TEXT) AS q, CAST(sqlc.arg(lang) AS TEXT) AS lang) AS p
WHERE make_key = sqlc.arg(make_key) AND (model_key = sqlc.arg(model_key) OR model_de_key = sqlc.arg(model_key))
  AND (instr(generation_key, p.q) > 0 OR instr(generation_de_key, p.q) > 0)
GROUP BY generation_key
ORDER BY substr(generation_key, 1, length(p.q)) <> p.q AND substr(generation_de_key, 1, length(p.q)) <> p.q,
         year_from IS NULL, year_from DESC, name COLLATE NOCASE
LIMIT 25;

-- name: CatalogEngines :many
SELECT CAST(MIN(engine) AS TEXT) AS name, CAST(MIN(engine_code) AS TEXT) AS engine_code,
       displacement_cc, power_kw, fuel_type
FROM catalog_vehicles, (SELECT CAST(sqlc.arg(q) AS TEXT) AS q) AS p
WHERE make_key = sqlc.arg(make_key) AND (model_key = sqlc.arg(model_key) OR model_de_key = sqlc.arg(model_key))
  AND (generation_key = sqlc.arg(generation_key) OR generation_de_key = sqlc.arg(generation_key))
  AND instr(engine_key, p.q) > 0
GROUP BY engine_key, displacement_cc, power_kw, fuel_type
ORDER BY substr(engine_key, 1, length(p.q)) <> p.q,
         name COLLATE NOCASE, power_kw
LIMIT 25;

-- name: GetCatalogMeta :one
SELECT * FROM catalog_meta WHERE id = 1;

-- name: UpdateCatalogMeta :exec
UPDATE catalog_meta SET seed_sha256 = ?, imported_at = ?, header_json = ?, source = ? WHERE id = 1;

-- name: SetCatalogBundledSha :exec
UPDATE catalog_meta SET bundled_sha256 = ? WHERE id = 1;

-- name: DeleteCatalogKBA :exec
DELETE FROM catalog_kba;

-- name: DeleteCatalogVehicles :exec
DELETE FROM catalog_vehicles;

-- name: InsertCatalogKBA :exec
INSERT INTO catalog_kba (hsn, tsn, make, model, model_de, trade_name, generation, engine_code, displacement_cc,
                         power_kw, fuel_type, body_code, year_from, source, match)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertCatalogVehicle :exec
INSERT INTO catalog_vehicles (make, make_key, model, model_key, model_de, model_de_key, generation, generation_key,
                              generation_de, generation_de_key, year_from, year_to, body, engine, engine_key,
                              engine_code, displacement_cc, power_kw, fuel_type)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
