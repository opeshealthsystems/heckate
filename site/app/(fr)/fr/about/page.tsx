import { AboutPage } from "@/components/pages";
import { pageMeta } from "@/components/seo";

export const metadata = pageMeta("fr", "about");

export default function Page() {
  return <AboutPage lang="fr" />;
}
