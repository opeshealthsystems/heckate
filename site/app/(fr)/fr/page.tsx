import { HomePage } from "@/components/pages";
import { pageMeta } from "@/components/seo";

export const metadata = pageMeta("fr", "home");

export default function Page() {
  return <HomePage lang="fr" />;
}
