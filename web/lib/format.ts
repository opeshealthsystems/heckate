/** XAF has no minor unit: amounts are whole francs. */
export const xaf = (n: number | null | undefined) =>
  n == null ? "–" : `${new Intl.NumberFormat("fr-FR").format(n)} XAF`;

export const day = (s: string | null | undefined) => (s ? new Date(s).toLocaleDateString() : "–");
export const when = (s: string | null | undefined) => (s ? new Date(s).toLocaleString() : "–");
export const roleLabel = (r: string) => (r === "ceo" ? "CEO" : r.charAt(0).toUpperCase() + r.slice(1).replace(/_/g, " "));
