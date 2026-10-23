import { View } from "react-native";
import { Redirect, Tabs, usePathname } from "expo-router";
import { Icon } from "../../components/Icon";
import { accessFor } from "../../lib/home-core";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";

// Web preview only: expo-router's own Tabs with default styling and system colours. The shipped app uses _layout.tsx (native tabs).
export default function TabsLayout() {
  const { me, ready } = useSession();
  const { c } = useTheme();
  const pathname = usePathname();
  if (ready && !me) return <Redirect href={{ pathname: "/login", params: { next: pathname } }} />;
  if (!ready) return <View style={{ flex: 1, backgroundColor: c.bg }} />;
  const staff = me?.scope === "staff";
  const access = me ? accessFor(me) : null;
  return (
    <Tabs screenOptions={{ headerShown: false, sceneStyle: { backgroundColor: c.bg as string }, tabBarActiveTintColor: c.tint as string, tabBarInactiveTintColor: c.label2 as string, tabBarStyle: { height: 64, paddingBottom: 8, backgroundColor: c.surface as string, borderTopColor: c.separator as string } }}>
      <Tabs.Screen name="home" options={{ title: staff ? "My day" : "Overview", href: access?.home ? undefined : null, tabBarIcon: ({ focused }) => <Icon name="home" size={24} tone={focused ? "tint" : "label2"} /> }} />
      <Tabs.Screen name="tickets" options={{ title: "Tickets", href: me?.scope === "other" ? null : undefined, tabBarIcon: ({ focused }) => <Icon name="tickets" size={24} tone={focused ? "tint" : "label2"} /> }} />
      <Tabs.Screen name="field" options={{ title: "Field", href: me?.roles.includes("engineer") ? undefined : null, tabBarIcon: ({ focused }) => <Icon name="field" size={24} tone={focused ? "tint" : "label2"} /> }} />
      <Tabs.Screen name="documents" options={{ title: "Documents", href: access?.documents ? undefined : null, tabBarIcon: ({ focused }) => <Icon name="documents" size={24} tone={focused ? "tint" : "label2"} /> }} />
      <Tabs.Screen name="account" options={{ title: "Account", tabBarIcon: ({ focused }) => <Icon name="account" size={24} tone={focused ? "tint" : "label2"} /> }} />
    </Tabs>
  );
}
