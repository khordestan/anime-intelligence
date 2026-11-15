// Exact selections deliberately accept explicit comma-separated integers only.
export function parseEpisodeNumbers(value: string): number[] {
  if (value.length > 70_000) throw new Error("Use at most 10,000 episode numbers.");
  if (!value.trim()) return [];
  const tokens = value.split(",");
  if (tokens.length > 10_000) throw new Error("Use at most 10,000 episode numbers.");
  const seen = new Set<number>();
  for (const token of tokens) {
    if (!/^\d+$/.test(token.trim())) throw new Error("Enter episode numbers separated by commas, such as 1, 2, 4.");
    const number = Number(token.trim());
    if (!Number.isSafeInteger(number) || number < 1 || number > 100_000 || seen.has(number)) throw new Error("Use unique episode numbers between 1 and 100,000.");
    seen.add(number);
  }
  return [...seen].sort((a, b) => a - b);
}
