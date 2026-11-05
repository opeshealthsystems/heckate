import { ContactPage } from "@/components/pages";
import { pageMeta } from "@/components/seo";

export const metadata = pageMeta("fr", "contact");

export default function Page() {
  return <ContactPage lang="fr" />;
}
