import { Text, View } from "react-native";
import { Button } from "../../components/ui";
import { useSession } from "../../lib/session";
import { useTheme } from "../../lib/theme";

export default function Account() {
  const { s } = useTheme();
  const { me, signOut } = useSession();
  return (
    <View style={[s.screen, s.pad]}>
      <Text style={s.h1}>Account</Text>
      <View style={s.card}>
        <Text style={s.text}>{me?.email || me?.sub}</Text>
        <Text style={s.muted}>{me?.roles.join(", ")}</Text>
      </View>
      <Button title="Sign out" onPress={() => void signOut()} />
    </View>
  );
}
