import { HomePage } from "@/components/pages";
import { pageMeta } from "@/components/seo";

export const metadata = pageMeta("en", "home");

export default function Page() {
  return <HomePage lang="en" />;
}
