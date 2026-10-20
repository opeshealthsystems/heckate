import { Redirect, Tabs } from "expo-router";
import { Icon, type IconName } from "../../components/Icon";
import { accessFor } from "../../lib/home-core";
import { useSession } from "../../lib/session";
import { font, useTheme } from "../../lib/theme";

const tabIcon = (name: IconName) => ({ color }: { color: unknown }) => <Icon name={name} color={String(color)} />;

export default function TabsLayout() {
  const { me, ready } = useSession();
  const { c } = useTheme();
  if (ready && !me) return <Redirect href="/login" />;
  const staff = me?.scope === "staff";
  const access = me ? accessFor(me) : null;
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
      <Tabs.Screen name="home" options={{ title: staff ? "My day" : "Overview", tabBarIcon: tabIcon("home"), href: access?.home ? undefined : null }} />
      <Tabs.Screen name="tickets" options={{ title: staff ? "Queue" : "Tickets", tabBarIcon: tabIcon("tickets") }} />
      <Tabs.Screen name="report" options={{ title: staff ? "New ticket" : "Report", tabBarIcon: tabIcon("plus"), href: me?.scope === "other" ? null : undefined }} />
      <Tabs.Screen name="field" options={{ title: "Field", tabBarIcon: tabIcon("field"), href: me?.roles.includes("engineer") ? undefined : null }} />
      <Tabs.Screen name="documents" options={{ title: "Documents", tabBarIcon: tabIcon("documents"), href: access?.documents ? undefined : null }} />
      <Tabs.Screen name="account" options={{ title: "Account", tabBarIcon: tabIcon("people") }} />
    </Tabs>
  );
}
