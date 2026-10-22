import { Redirect } from "expo-router";
import { Text, View } from "react-native";
import { Backdrop, Emblem } from "../components/ui";
import { useSession } from "../lib/session";
import { useTheme } from "../lib/theme";

export default function Index() {
  const { me, ready } = useSession();
  const { s, c } = useTheme();
  if (!ready) {
    return (
      <View style={{ flex: 1, backgroundColor: c.void, alignItems: "center", justifyContent: "center", gap: 14 }}>
        <Backdrop />
        <Emblem size={96} />
        <Text style={s.overline}>Restoring secure session</Text>
      </View>
    );
  }
  return <Redirect href={me ? "/(tabs)/home" : "/login"} />;
}
