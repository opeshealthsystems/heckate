import { View } from "react-native";
import { Redirect, Tabs, usePathname } from "expo-router";
import { TabBar } from "../../components/TabBar";
import { accessFor } from "../../lib/home-core";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";

export default function TabsLayout() {
  const { me, ready } = useSession();
  const { c } = useTheme();
  const pathname = usePathname();
  if (ready && !me) return <Redirect href={{ pathname: "/login", params: { next: pathname } }} />;
  // Hold the tab navigator until the session is known. Tabs are hidden with `href: null` by role, and expo-router
  // resolves the URL against the tabs that exist when the navigator mounts: mounted before the role is known, the
  // Home and Documents tabs were missing, so a deep link to /home or /documents fell through to the Queue tab.
  if (!ready) return <View style={{ flex: 1, backgroundColor: c.bg }} />;
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
