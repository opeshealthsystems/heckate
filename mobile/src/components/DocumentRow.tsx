import { View } from "react-native";
import { router } from "expo-router";
import type { DocumentEntry } from "../lib/types";
import { Icon } from "./Icon";
import { Row } from "./List";
import { Text } from "./Text";

export const docDay = (s: string) => new Date(s).toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" });

/** A register entry: reference (monospace), title, then "Valid" as plain text with a small check, the code and the date. Opens the verification. */
export function DocumentRow({ doc: d }: { doc: DocumentEntry }) {
  const valid = d.status === "valid";
  return (
    <Row
      mono
      title={d.ref}
      titleLines={1}
      accessibilityLabel={`${d.title}, ${valid ? "valid" : "revoked"}`}
      subtitle={
        <View style={{ gap: 2 }}>
          <Text v="subhead" numberOfLines={2}>{d.title}</Text>
          <View style={{ flexDirection: "row", alignItems: "center", gap: 4 }}>
            {valid && <Icon name="check" size={14} tone="label2" />}
            <Text v="subhead" tone={valid ? "label2" : "red"} numberOfLines={1} style={{ flexShrink: 1 }}>{`${valid ? "Valid" : "Revoked"} · ${docDay(d.issued_at)}`}</Text>
          </View>
        </View>
      }
      onPress={() => router.push(`/document/${encodeURIComponent(d.ref)}`)}
      chevron
    />
  );
}
