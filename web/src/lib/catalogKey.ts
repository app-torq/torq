/**
 * Mirrors internal/catalog.Key: case and accents are ignored, "ß" matches
 * "ss", and spaces, hyphens, dots and slashes are dropped.
 */
export function catalogKey(s: string): string {
  return s
    .normalize('NFKD')
    .replace(/\p{Mn}/gu, '')
    .toLowerCase()
    .replace(/ß/g, 'ss')
    .replace(/[\s\-./]/g, '')
}
