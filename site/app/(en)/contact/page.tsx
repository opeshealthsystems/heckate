import { ContactPage } from "@/components/pages";
import { pageMeta } from "@/components/seo";

export const metadata = pageMeta("en", "contact");

export default function Page() {
  return <ContactPage lang="en" />;
}
