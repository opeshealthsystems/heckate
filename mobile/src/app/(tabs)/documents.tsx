import { useCallback, useState } from "react";
import { FlatList, Pressable, RefreshControl, Text, TextInput, View } from "react-native";
import { useFocusEffect } from "expo-router";
import { Pill } from "../../components/ui";
import { api } from "../../lib/api";
import { accessFor, filterRegister } from "../../lib/home-core";
import { useSession } from "../../lib/session";
import { font, useTheme } from "../../lib/theme";
import type { DocumentEntry, Verification } from "../../lib/types";

export default function Documents() {
  const { s, c } = useTheme();
  const { me } = useSession();
  const [rows, setRows] = useState<DocumentEntry[]>([]);
  const [q, setQ] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [checking, setChecking] = useState<string | null>(null);
  const [results, setResults] = useState<Record<string, Verification | string>>({});

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setRows((await api.documents()).data);
      setErr("");
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
  }, []);
  useFocusEffect(useCallback(() => void load(), [load]));

  async function verify(ref: string) {
    setChecking(ref);
    try {
      const v = await api.verify(ref);
      setResults((p) => ({ ...p, [ref]: v }));
    } catch (e) {
      setResults((p) => ({ ...p, [ref]: (e as Error).message }));
    }
    setChecking(null);
  }

  if (!me || !accessFor(me).documents) return <View style={s.pad}><Text style={s.text}>Your role does not have access to documents.</Text></View>;
  const shown = filterRegister(rows, q);
  return (
    <FlatList
      style={s.screen}
      contentContainerStyle={s.pad}
      data={shown}
      keyExtractor={(d) => d.ref}
      refreshControl={<RefreshControl refreshing={busy} onRefresh={load} />}
      ListHeaderComponent={
        <View style={{ gap: 8 }}>
          <Text style={s.muted}>Every issued document is numbered and can be checked by its reference.</Text>
          <TextInput accessibilityLabel="Search the register" style={s.input} placeholder="Search by reference, code or title" placeholderTextColor={c.muted} value={q} onChangeText={setQ} autoCapitalize="none" autoCorrect={false} />
          {!!err && <View style={[s.card, { borderColor: c.bad }]}><Text style={s.error}>Could not load: {err}</Text><Text style={s.muted}>Pull down to try again.</Text></View>}
        </View>
      }
      ListEmptyComponent={busy || err ? undefined : <View style={s.card}><Text style={s.text}>{q ? "Nothing matches." : "No documents issued yet."}</Text></View>}
      renderItem={({ item: d }) => {
        const r = results[d.ref];
        return (
          <View style={s.card}>
            <Text style={{ color: c.goldText, fontSize: 13, fontFamily: font.semibold }} selectable>{d.ref}</Text>
            <Text style={[s.text, s.semibold]}>{d.title}</Text>
            <View style={s.row}>
              <Pill text={d.status} tone={d.status === "valid" ? "ok" : "bad"} />
              <Text style={s.muted}>{d.code} · {new Date(d.issued_at).toLocaleDateString()}</Text>
            </View>
            <Pressable accessibilityRole="button" disabled={checking === d.ref} onPress={() => void verify(d.ref)} style={{ alignSelf: "flex-start", marginTop: 6, minHeight: 44, justifyContent: "center" }}>
              <Text style={{ color: c.brand, fontSize: 15, fontFamily: font.semibold, textDecorationLine: "underline" }}>{checking === d.ref ? "Checking…" : "Verify this document"}</Text>
            </Pressable>
            {typeof r === "string" && !!r && <Text style={s.error}>{r}</Text>}
            {r && typeof r === "object" && (
              <Text style={{ color: r.valid ? c.ok : c.bad, fontSize: 14, fontFamily: font.semibold }} accessibilityLiveRegion="polite">
                {r.valid ? `Authentic${r.issued_at ? ` · issued ${new Date(r.issued_at).toLocaleDateString()}` : ""}` : `Not valid${r.reason ? `: ${r.reason}` : ""}`}
              </Text>
            )}
          </View>
        );
      }}
    />
  );
}
