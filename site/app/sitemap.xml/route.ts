import { company } from "@/content/company";
import { PAGES, sitemapXml } from "@/content/seo";

export const dynamic = "force-static";

export function GET() {
  return new Response(sitemapXml(company.url, PAGES), { headers: { "Content-Type": "application/xml" } });
}
