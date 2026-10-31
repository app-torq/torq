// i18n gate (Constitution V):
//  1. every key in the English reference locale exists in German and vice versa;
//  2. every API error code the Go backend can return has a translation in the
//     errors namespace, so no raw codes ever reach the UI.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

const webRoot = join(import.meta.dirname, '..')
const localesDir = join(webRoot, 'src', 'i18n', 'locales')
const goRoot = join(webRoot, '..', 'internal')

function flatten(obj: unknown, prefix = ''): string[] {
  if (obj === null || typeof obj !== 'object') return [prefix]
  return Object.entries(obj as Record<string, unknown>).flatMap(([k, v]) => flatten(v, prefix ? `${prefix}.${k}` : k))
}

function keys(locale: string): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>()
  for (const file of readdirSync(join(localesDir, locale))) {
    if (!file.endsWith('.json')) continue
    const data: unknown = JSON.parse(readFileSync(join(localesDir, locale, file), 'utf8'))
    out.set(file.replace(/\.json$/, ''), new Set(flatten(data).filter(Boolean)))
  }
  return out
}

const problems: string[] = []
const en = keys('en')
const de = keys('de')
for (const [ns, enKeys] of en) {
  const deKeys = de.get(ns)
  if (!deKeys) {
    problems.push(`de: missing namespace ${ns}`)
    continue
  }
  for (const k of enKeys) if (!deKeys.has(k)) problems.push(`de: missing ${ns}:${k}`)
  for (const k of deKeys) if (!enKeys.has(k)) problems.push(`en: missing ${ns}:${k} (present in de)`)
}
for (const ns of de.keys()) if (!en.has(ns)) problems.push(`en: missing namespace ${ns}`)

// Collect error codes from Go sources: apperr constructors and Validation.Add/Check.
function goFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) return goFiles(p)
    return p.endsWith('.go') && !p.endsWith('_test.go') && !p.includes('/store/') && !p.includes('/db/') ? [p] : []
  })
}
// One pattern per call shape; the captured group is always the code.
const codePatterns = [
  /apperr\.\w+\("([^"]+)"/g, // apperr.Conflict("code", ...)
  /\bCode:\s*"([^"]+)"/g, // &apperr.Error{Code: "code"} / errorPayload{Code: "code"} (not BodyCode:)
  /\.Add\("[^"]*",\s*"([^"]+)"\)/g, // v.Add("field", "code")
  /\.Check\([^\n]*?,\s*"[^"]*",\s*"([^"]+)"\)/g, // v.Check(cond, "field", "code")
]
const codes = new Set<string>()
for (const file of goFiles(goRoot)) {
  const src = readFileSync(file, 'utf8')
  for (const re of codePatterns) for (const m of src.matchAll(re)) if (m[1]) codes.add(m[1])
}
for (const locale of ['en', 'de'] as const) {
  const errorKeys = (locale === 'en' ? en : de).get('errors') ?? new Set<string>()
  for (const code of codes) if (!errorKeys.has(code)) problems.push(`${locale}: errors:${code} missing (returned by the API)`)
}

if (problems.length > 0) {
  console.error(problems.join('\n'))
  console.error(`\n${problems.length} i18n problem(s)`)
  process.exit(1)
}
let total = 0
for (const s of en.values()) total += s.size
console.log(`i18n OK: ${en.size} namespaces, ${total} keys in en and de; ${codes.size} API error codes translated`)
