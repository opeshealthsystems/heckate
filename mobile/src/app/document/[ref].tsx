import { useCallback, useEffect, useState } from "react";
import { View } from "react-native";
import { Stack, useLocalSearchParams } from "expo-router";
import { Banner } from "../../components/Banner";
import { Button } from "../../components/Button";
import { docDay } from "../../components/DocumentRow";
import { Loading } from "../../components/EmptyState";
import { useGate } from "../../components/Header";
import { Icon } from "../../components/Icon";
import { Row, Section } from "../../components/List";
import { Screen } from "../../components/Screen";
import { Text } from "../../components/Text";
import { api } from "../../lib/api";
import { haptic } from "../../lib/haptics";
import { sp } from "../../lib/theme";
import type { Verification } from "../../lib/types";

type Result = { v: Verification } | { error: string };

/** The verification verdict. A large icon and one word carry the result: green only for "Authentic", red for "Not valid". */
export default function DocumentScreen() {
  const gate = useGate();
  const { ref: raw } = useLocalSearchParams<{ ref: string }>();
  const ref = decodeURIComponent(raw ?? "");
  const [r, setR] = useState<Result | null>(null);

  const check = useCallback(async () => {
    setR(null);
    try {
      const v = await api.verify(ref);
      (v.valid ? haptic.success : haptic.error)();
      setR({ v });
    } catch (e) {
      haptic.warning();
      setR({ error: (e as Error).message });
    }
  }, [ref]);
  useEffect(() => { void check(); }, [check]);
  if (gate) return gate;

  return (
    <Screen webTitle="Document">
      <Stack.Screen options={{ title: "Document" }} />
      {!r && <Loading />}
      {r && "error" in r && (
        <>
          <Banner icon="warning" tone="red">Could not check this document. No verdict was reached. Try again when you are online.</Banner>
          <Button title="Try again" onPress={() => void check()} />
        </>
      )}
      {r && "v" in r && (
        <>
          <View accessibilityRole="alert" accessibilityLabel={r.v.valid ? "Authentic document" : "Not valid"} style={{ alignItems: "center", gap: sp.sm, paddingVertical: sp.lg }}>
            <Icon name={r.v.valid ? "verified" : "invalid"} size={72} tone={r.v.valid ? "green" : "red"} />
            <Text v="title" tone={r.v.valid ? "green" : "red"} style={{ marginTop: sp.sm }}>{r.v.valid ? "Authentic" : "Not valid"}</Text>
            <Text v="subhead" tone="label2" style={{ textAlign: "center", maxWidth: 320 }}>
              {r.v.valid ? "This reference exists in the HEKATE register and is currently in force." : r.v.reason ? r.v.reason : "This reference is not in force or is not in the HEKATE register."}
            </Text>
          </View>
          <Section header="Facts">
            <Row title="Reference" right={<Text mono tone="label2" selectable numberOfLines={1} style={{ flexShrink: 1 }}>{r.v.ref ?? ref}</Text>} />
            {!!r.v.title && <Row title="Title" value={r.v.title} />}
            {!!r.v.code && <Row title="Code" right={<Text mono tone="label2" selectable>{r.v.code}</Text>} />}
            {!!r.v.issued_at && <Row title="Issued" value={docDay(r.v.issued_at)} />}
            {!!r.v.status && <Row title="Status" value={r.v.status === "valid" ? "Valid" : r.v.status === "revoked" ? "Revoked" : r.v.status} />}
          </Section>
        </>
      )}
    </Screen>
  );
}
