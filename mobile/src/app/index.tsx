import { Redirect } from "expo-router";
import { Text, View } from "react-native";
import { useSession } from "../lib/session";
import { useTheme } from "../lib/theme";

export default function Index() {
  const { me, ready } = useSession();
  const { s } = useTheme();
  if (!ready) return <View style={[s.screen, { justifyContent: "center", alignItems: "center" }]}><Text style={s.muted}>Loading…</Text></View>;
  return <Redirect href={me ? "/(tabs)/home" : "/login"} />;
}
