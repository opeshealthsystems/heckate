export type DiffLine = { kind: "same" | "add" | "del"; text: string };

/** Line-by-line difference between two texts (longest common subsequence), for showing what a version changed. */
export function diffLines(before: string, after: string): DiffLine[] {
  const a = before.split("\n"), b = after.split("\n");
  const n = a.length, m = b.length;
  const l: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0));
  for (let i = n - 1; i >= 0; i--) for (let j = m - 1; j >= 0; j--) l[i][j] = a[i] === b[j] ? l[i + 1][j + 1] + 1 : Math.max(l[i + 1][j], l[i][j + 1]);
  const out: DiffLine[] = [];
  let i = 0, j = 0;
  while (i < n && j < m) {
    if (a[i] === b[j]) { out.push({ kind: "same", text: a[i] }); i++; j++; }
    else if (l[i + 1][j] >= l[i][j + 1]) out.push({ kind: "del", text: a[i++] });
    else out.push({ kind: "add", text: b[j++] });
  }
  while (i < n) out.push({ kind: "del", text: a[i++] });
  while (j < m) out.push({ kind: "add", text: b[j++] });
  return out;
}

/** A version's content as plain text, the same way for every version so two of them can be compared. */
export function versionText(v: { title: string; signed_by: string | null; sections: { heading: string; body: string }[] }): string {
  return [`Title: ${v.title}`, ...(v.signed_by ? [`Signed for HEKATE by: ${v.signed_by}`] : []), "", ...v.sections.flatMap((s) => [...(s.heading ? [`## ${s.heading}`] : []), s.body, ""])].join("\n").trimEnd();
}
