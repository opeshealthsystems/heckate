import { useState } from "react";
import { Text, View } from "react-native";
import { Button } from "./ui";
import { uploadPhoto } from "../lib/api";
import { pickPhoto } from "../lib/photos";
import { useTheme } from "../lib/theme";

/** Camera and library buttons that attach a photo to a record and report the result. */
export function PhotoButtons({ entityType, entityId, onUploaded }: { entityType: "ticket" | "intervention" | "expense"; entityId: string; onUploaded?: () => void }) {
  const { s } = useTheme();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState("");
  async function add(source: "camera" | "library") {
    setBusy(true);
    setMsg("");
    try {
      const photo = await pickPhoto(source);
      if (photo) {
        await uploadPhoto(entityType, entityId, photo);
        setMsg("Photo attached.");
        onUploaded?.();
      }
    } catch (e) {
      setMsg((e as Error).message);
    }
    setBusy(false);
  }
  return (
    <View style={{ gap: 8 }}>
      <View style={s.row}>
        <Button title="Take a photo" disabled={busy} onPress={() => void add("camera")} />
        <Button title="Choose a photo" disabled={busy} onPress={() => void add("library")} />
      </View>
      {!!msg && <Text style={msg === "Photo attached." ? s.muted : s.error}>{msg}</Text>}
    </View>
  );
}
