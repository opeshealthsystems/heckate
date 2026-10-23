import { useCallback, useState } from "react";
import { Platform } from "react-native";
import { router, Stack, useFocusEffect } from "expo-router";
import { ErrorBanner } from "../../../components/Banner";
import { DocumentRow } from "../../../components/DocumentRow";
import { EmptyState, Loading } from "../../../components/EmptyState";
import { Row, Section } from "../../../components/List";
import { Screen } from "../../../components/Screen";
import { Field } from "../../../components/TextField";
import { api } from "../../../lib/api";
import { accessFor, filterRegister } from "../../../lib/home-core";
import { useSession } from "../../../lib/session";
import type { DocumentEntry } from "../../../lib/types";

export default function Documents() {
  const { me } = useSession();
  const [rows, setRows] = useState<DocumentEntry[]>([]);
  const [q, setQ] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setRows((await api.documents()).data);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
    setLoaded(true);
  }, []);
  useFocusEffect(useCallback(() => void load(), [load]));

  if (!me || !accessFor(me).documents) {
    return <Screen><EmptyState icon="lock" title="No access" text="Your role does not have access to documents." /></Screen>;
  }
  const shown = filterRegister(rows, q);
  const typed = q.trim();
  return (
    <Screen webTitle="Documents" refreshing={busy && loaded} onRefresh={load}>
      <Stack.Screen
        options={{
          title: "Documents",
          headerSearchBarOptions: { placeholder: "Reference, code or title", autoCapitalize: "none", hideWhenScrolling: false, onChangeText: (e) => setQ(e.nativeEvent.text) },
        }}
      />
      {Platform.OS === "web" && <Section><Field placeholder="Search by reference, code or title" value={q} onChangeText={setQ} autoCapitalize="none" autoCorrect={false} returnKeyType="search" /></Section>}
      {!!err && <ErrorBanner message={err} />}
      {!loaded && busy ? <Loading /> : shown.length > 0 ? (
        <Section footer={me.scope === "client" ? "Documents issued to your company. Open one to check that it is authentic." : "Every issued document is numbered. Open one to check that it is authentic."}>
          {shown.map((d) => <DocumentRow key={d.ref} doc={d} />)}
        </Section>
      ) : loaded && !err ? (
        <>
          {!!typed && <Section footer="Checks a reference against the HEKATE register."><Row icon="search" action title={`Verify “${typed}”`} onPress={() => router.push(`/document/${encodeURIComponent(typed)}`)} /></Section>}
          <EmptyState icon="documents" title={typed ? "Nothing matches" : "No documents yet"} text={typed ? "Check the reference, or verify it directly." : "Issued documents appear here."} />
        </>
      ) : null}
    </Screen>
  );
}
