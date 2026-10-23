import { View } from "react-native";
import { MaterialIcons } from "@expo/vector-icons";
import { Redirect, usePathname } from "expo-router";
import { NativeTabs } from "expo-router/native-tabs";
import { accessFor } from "../../lib/home-core";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";

// Real native tabs: UITabBarController on iOS, Material bottom navigation on Android.
// (The web preview uses _layout.web.tsx, because native tabs have no icons or styling there.)
export default function TabsLayout() {
  const { me, ready } = useSession();
  const { c } = useTheme();
  const pathname = usePathname();
  if (ready && !me) return <Redirect href={{ pathname: "/login", params: { next: pathname } }} />;
  // Hold the tab navigator until the session is known: tabs are hidden by role, and the URL is resolved against the
  // tabs that exist when the navigator mounts. Mounted before the role is known, a deep link to /home or /documents
  // fell through to the first tab.
  if (!ready) return <View style={{ flex: 1, backgroundColor: c.bg }} />;
  const staff = me?.scope === "staff";
  const access = me ? accessFor(me) : null;
  const icon = (sf: string, selected: string, md: React.ComponentProps<typeof MaterialIcons>["name"]) => (
    <NativeTabs.Trigger.Icon sf={{ default: sf as never, selected: selected as never }} src={<NativeTabs.Trigger.VectorIcon family={MaterialIcons} name={md} />} />
  );
  return (
    <NativeTabs tintColor={c.tint} badgeBackgroundColor={c.red}>
      <NativeTabs.Trigger name="home" hidden={!access?.home}>
        <NativeTabs.Trigger.Label>{staff ? "My day" : "Overview"}</NativeTabs.Trigger.Label>
        {icon("house", "house.fill", "home")}
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="tickets" hidden={me?.scope === "other"}>
        <NativeTabs.Trigger.Label>Tickets</NativeTabs.Trigger.Label>
        {icon("ticket", "ticket.fill", "confirmation-number")}
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="field" hidden={!me?.roles.includes("engineer")}>
        <NativeTabs.Trigger.Label>Field</NativeTabs.Trigger.Label>
        {icon("mappin.and.ellipse", "mappin.and.ellipse", "place")}
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="documents" hidden={!access?.documents}>
        <NativeTabs.Trigger.Label>Documents</NativeTabs.Trigger.Label>
        {icon("doc.text", "doc.text.fill", "description")}
      </NativeTabs.Trigger>
      <NativeTabs.Trigger name="account">
        <NativeTabs.Trigger.Label>Account</NativeTabs.Trigger.Label>
        {icon("person.crop.circle", "person.crop.circle.fill", "account-circle")}
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}
