import { View } from "react-native";
import { router } from "expo-router";
import type { DocumentEntry } from "../lib/types";
import { Row } from "./List";
import { Text } from "./Text";

export const docDay = (s: string) => new Date(s).toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });

/**
 * A register entry: the title is the main line, the reference sits under it (13 pt monospace, secondary), the date trails.
 * Valid is the normal case and is not repeated; only an exception (Revoked) is shown, in red. Opens the verification.
 */
export function DocumentRow({ doc: d }: { doc: DocumentEntry }) {
  const valid = d.status === "valid";
  return (
    <Row
      title={d.title}
      titleLines={2}
      accessibilityLabel={`${d.title}, ${d.ref}, ${valid ? "valid" : "revoked"}`}
      subtitle={
        <View style={{ gap: 2 }}>
          <Text mono v="footnote" tone="label2" numberOfLines={1}>{d.ref}</Text>
          {!valid && <Text v="footnote" tone="red">Revoked</Text>}
        </View>
      }
      value={new Date(d.issued_at).toLocaleDateString(undefined, { dateStyle: "short" })}
      
      onPress={() => router.push(`/document/${encodeURIComponent(d.ref)}`)}
      chevron
    />
  );
}
