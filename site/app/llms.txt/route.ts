import { company } from "@/content/company";
import { en } from "@/content/en";
import { llmsTxt } from "@/content/seo";

export const dynamic = "force-static";

export function GET() {
  return new Response(llmsTxt(company, en), { headers: { "Content-Type": "text/plain; charset=utf-8" } });
}
