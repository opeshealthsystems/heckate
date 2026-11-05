import { ServicesPage } from "@/components/pages";
import { pageMeta } from "@/components/seo";

export const metadata = pageMeta("en", "services");

export default function Page() {
  return <ServicesPage lang="en" />;
}
