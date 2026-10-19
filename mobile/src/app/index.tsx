import { Redirect } from "expo-router";
import { Text, View } from "react-native";
import { useSession } from "../lib/session";

export default function Index() {
  const { me, ready } = useSession();
  if (!ready) return <View style={{ flex: 1, justifyContent: "center", alignItems: "center" }}><Text>Loading…</Text></View>;
  return <Redirect href={me ? "/(tabs)/tickets" : "/login"} />;
}
