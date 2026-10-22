import { Redirect, Tabs } from "expo-router";
import { TabBar } from "../../components/TabBar";
import { accessFor } from "../../lib/home-core";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";

export default function TabsLayout() {
  const { me, ready } = useSession();
  const { c } = useTheme();
  if (ready && !me) return <Redirect href="/login" />;
  const staff = me?.scope === "staff";
  const access = me ? accessFor(me) : null;
  return (
    <Tabs tabBar={(p) => <TabBar {...p} />} screenOptions={{ headerShown: false, sceneStyle: { backgroundColor: c.bg } }}>
      <Tabs.Screen name="home" options={{ title: staff ? "My day" : "Overview", href: access?.home ? undefined : null }} />
      <Tabs.Screen name="tickets" options={{ title: staff ? "Queue" : "Tickets" }} />
      <Tabs.Screen name="report" options={{ title: staff ? "New" : "Report", href: me?.scope === "other" ? null : undefined }} />
      <Tabs.Screen name="field" options={{ title: "Field", href: me?.roles.includes("engineer") ? undefined : null }} />
      <Tabs.Screen name="documents" options={{ title: "Docs", href: access?.documents ? undefined : null }} />
      <Tabs.Screen name="account" options={{ title: "Account" }} />
    </Tabs>
  );
}
