import { en } from "./en";
import { fr } from "./fr";
import type { Dict, Lang } from "./types";

export const dicts: Record<Lang, Dict> = { en, fr };
export const path = (lang: Lang, p: string) => (lang === "en" ? p : `/fr${p === "/" ? "" : p}`);
