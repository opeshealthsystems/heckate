import type { Metadata } from "next";
import { company } from "@/content/company";
import { dicts, path } from "@/content";
import type { Lang } from "@/content/types";

export type PageKey = "home" | "services" | "about" | "contact" | "guarantees";
const route: Record<PageKey, string> = { home: "/", services: "/services/", about: "/about/", contact: "/contact/", guarantees: "/guarantees/" };

export function pageMeta(lang: Lang, key: PageKey): Metadata {
  const d = dicts[lang];
  const p = key === "home" ? d.home : d[key];
  const url = (l: Lang) => `${company.url}${path(l, route[key])}`;
  return {
    metadataBase: new URL(company.url),
    title: p.title,
    description: p.description,
    alternates: { canonical: url(lang), languages: { en: url("en"), fr: url("fr"), "x-default": url("en") } },
    openGraph: { title: p.title, description: p.description, url: url(lang), siteName: company.brand, locale: d.locale, type: "website" },
    robots: { index: true, follow: true },
    icons: { icon: [{ url: "/favicon-32.png", sizes: "32x32" }, { url: "/icon-192.png", sizes: "192x192" }], apple: "/icon-192.png" },
  };
}

export function JsonLd({ data }: { data: unknown }) {
  return <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(data).replace(/</g, "\\u003c") }} />;
}
