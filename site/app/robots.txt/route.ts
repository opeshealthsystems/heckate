import { company } from "@/content/company";

export const dynamic = "force-static";

export function GET() {
  return new Response(`User-agent: *\nAllow: /\n\nSitemap: ${company.url}/sitemap.xml\n`, { headers: { "Content-Type": "text/plain" } });
}
