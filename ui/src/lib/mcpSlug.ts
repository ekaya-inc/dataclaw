export function toMCPKey(name: string): string {
  const lower = name.toLowerCase();
  const slug = lower.replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '');
  return slug === '' ? 'agent' : slug;
}
