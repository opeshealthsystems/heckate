import { ActivityIndicator, View } from "react-native";
import { Redirect } from "expo-router";
import { useSession } from "../lib/session";
import { useTheme } from "../lib/theme";

export default function Index() {
  const { me, ready } = useSession();
  const { c } = useTheme();
  if (!ready) {
    return (
      <View style={{ flex: 1, backgroundColor: c.bg, alignItems: "center", justifyContent: "center" }}>
        <ActivityIndicator />
      </View>
    );
  }
  return <Redirect href={me ? "/(tabs)/home" : "/login"} />;
}
