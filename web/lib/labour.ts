/** "N approved hours have no rate" – the unpriced labour note, or null when every approved hour is priced. */
export function unpricedNote(hours: number): string | null {
  if (!(hours > 0)) return null;
  const h = new Intl.NumberFormat("en", { maximumFractionDigits: 2 }).format(hours);
  return `${h} approved ${hours === 1 ? "hour has" : "hours have"} no rate, so ${hours === 1 ? "it is" : "they are"} not in the actual cost.`;
}
