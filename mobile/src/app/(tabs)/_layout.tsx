import { Redirect, Tabs } from "expo-router";
import { useSession } from "../../lib/session";
import { font, useTheme } from "../../lib/theme";

export default function TabsLayout() {
  const { me, ready } = useSession();
  const { c } = useTheme();
  if (ready && !me) return <Redirect href="/login" />;
  const staff = me?.scope === "staff";
  return (
    <Tabs
      screenOptions={{
        tabBarActiveTintColor: c.brand,
        tabBarInactiveTintColor: c.muted,
        tabBarLabelStyle: { fontFamily: font.medium },
        headerStyle: { backgroundColor: c.brand },
        headerTintColor: "#ffffff",
        headerTitleStyle: { fontFamily: font.semibold },
        tabBarStyle: { backgroundColor: c.card, borderTopColor: c.line },
      }}
    >
      <Tabs.Screen name="tickets" options={{ title: staff ? "Queue" : "Tickets" }} />
      <Tabs.Screen name="report" options={{ title: staff ? "New ticket" : "Report", href: me?.scope === "other" ? null : undefined }} />
      <Tabs.Screen name="field" options={{ title: "Field", href: me?.roles.includes("engineer") ? undefined : null }} />
      <Tabs.Screen name="account" options={{ title: "Account" }} />
    </Tabs>
  );
}
