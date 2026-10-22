import { useCallback, useRef, useState } from "react";
import { FlatList, RefreshControl, Text, View } from "react-native";
import { useFocusEffect } from "expo-router";
import { Icon } from "../../components/Icon";
import { Button, EmptyState, ErrorState, Input, PressCard, Pill, Screen, ScreenHeader, SkeletonRows } from "../../components/ui";
import { api } from "../../lib/api";
import { accessFor, filterRegister } from "../../lib/home-core";
import { haptic } from "../../lib/haptics";
import { FadeIn } from "../../lib/motion";
import { useSession } from "../../lib/session";
import { font, radius, sp, useTheme } from "../../lib/theme";
import type { DocumentEntry, Verification } from "../../lib/types";

type Result = { v: Verification } | { error: string };
const day = (s: string) => new Date(s).toLocaleDateString("en-GB", { day: "2-digit", month: "short", year: "numeric" });

/** The verdict: a large, unmistakable panel. The word and the icon carry the result, not only the colour. */
function Verdict({ r, reference, onClose }: { r: Result; reference: string; onClose: () => void }) {
  const { c, s } = useTheme();
  if ("error" in r) {
    return (
      <View accessibilityRole="alert" style={{ backgroundColor: c.warnBg, borderColor: c.warn, borderWidth: 1, borderRadius: radius.md, padding: sp.md, gap: 4 }}>
        <Text style={[s.overline, { color: c.warn }]}>Could not check</Text>
        <Text style={[s.mono, { color: c.text2 }]} selectable>{r.error}</Text>
        <Text style={s.muted}>No verdict was reached. Try again when you are online.</Text>
      </View>
    );
  }
  const ok = r.v.valid;
  const col = ok ? c.ok : c.bad;
  return (
    <View accessibilityRole="alert" accessibilityLiveRegion="polite" accessibilityLabel={ok ? "Authentic document" : "Not valid"} style={{ backgroundColor: ok ? c.okBg : c.badBg, borderColor: col, borderWidth: 1.5, borderRadius: radius.lg, padding: sp.lg, gap: sp.sm, overflow: "hidden" }}>
      <View style={{ position: "absolute", left: 0, top: 0, bottom: 0, width: 5, backgroundColor: col }} />
      <View style={{ flexDirection: "row", alignItems: "center", gap: sp.md }}>
        <View style={{ width: 52, height: 52, borderRadius: 26, borderWidth: 2, borderColor: col, alignItems: "center", justifyContent: "center" }}>
          <Icon name={ok ? "shieldCheck" : "docX"} size={28} color={col} stroke={2} />
        </View>
        <View style={{ flex: 1 }}>
          <Text style={[s.overline, { color: col }]}>Verification result</Text>
          <Text style={{ color: col, fontSize: 24, lineHeight: 30, fontFamily: font.display }}>{ok ? "Authentic" : "Not valid"}</Text>
        </View>
        <Button small variant="ghost" title="Dismiss" onPress={onClose} />
      </View>
      <Text style={[s.mono, { color: c.fg, fontSize: 13 }]} selectable>{r.v.ref ?? reference}</Text>
      {ok ? (
        <>
          {!!r.v.title && <Text style={[s.text, s.semibold]}>{r.v.title}</Text>}
          <Text style={[s.mono, { color: c.text2 }]}>{[r.v.code, r.v.issued_at ? `issued ${day(r.v.issued_at)}` : "", r.v.status].filter(Boolean).join(" · ")}</Text>
          <Text style={[s.muted, { fontSize: 12 }]}>This reference exists in the HEKATE register and is currently in force.</Text>
        </>
      ) : (
        <Text style={[s.text, { color: c.fg }]}>{r.v.reason ? r.v.reason : "This reference is not in force or is not in the HEKATE register."}</Text>
      )}
    </View>
  );
}

export default function Documents() {
  const { s, c } = useTheme();
  const { me } = useSession();
  const [rows, setRows] = useState<DocumentEntry[]>([]);
  const [q, setQ] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [checking, setChecking] = useState<string | null>(null);
  const [result, setResult] = useState<{ ref: string; r: Result } | null>(null);
  const [updated, setUpdated] = useState<Date | null>(null);
  const list = useRef<FlatList<DocumentEntry>>(null);

  const load = useCallback(async () => {
    setBusy(true);
    try {
      setRows((await api.documents()).data);
      setErr("");
      setUpdated(new Date());
    } catch (e) {
      setErr((e as Error).message);
    }
    setBusy(false);
    setLoaded(true);
  }, []);
  useFocusEffect(useCallback(() => void load(), [load]));

  async function verify(ref: string) {
    setChecking(ref);
    setResult(null);
    try {
      const v = await api.verify(ref);
      (v.valid ? haptic.success : haptic.error)();
      setResult({ ref, r: { v } });
    } catch (e) {
      haptic.warning();
      setResult({ ref, r: { error: (e as Error).message } });
    }
    setChecking(null);
    list.current?.scrollToOffset({ offset: 0, animated: true });
  }

  if (!me || !accessFor(me).documents) {
    return <Screen><View style={s.pad}><EmptyState icon="lock" title="No access" text="Your role does not have access to documents." /></View></Screen>;
  }
  const shown = filterRegister(rows, q);
  const typed = q.trim();
  return (
    <Screen updated={updated}>
      <FlatList
        ref={list}
        contentContainerStyle={{ padding: sp.lg, gap: sp.sm }}
        data={loaded ? shown : []}
        keyExtractor={(d) => d.ref}
        keyboardShouldPersistTaps="handled"
        refreshControl={<RefreshControl refreshing={busy && loaded} onRefresh={load} tintColor={c.gold} colors={[c.gold]} progressBackgroundColor={c.surface} />}
        ListHeaderComponent={
          <View style={{ gap: sp.md, marginBottom: sp.sm }}>
            <ScreenHeader overline="Register and verification" title="Documents" />
            <Text style={s.muted}>{me.scope === "client" ? "Documents issued to your company. Each is numbered and can be checked by its reference." : "Every issued document is numbered and can be checked by its reference."}</Text>
            <Input label="Search or verify" accessibilityLabel="Search the register" placeholder="Reference, code or title" mono value={q} onChangeText={setQ} autoCapitalize="none" autoCorrect={false} />
            {!!typed && !shown.length && <Button icon="scan" title={`Verify “${typed}” directly`} loading={checking === typed} onPress={() => void verify(typed)} />}
            {result && <Verdict r={result.r} reference={result.ref} onClose={() => setResult(null)} />}
            {!!err && <ErrorState message={err} onRetry={load} />}
            {!loaded && busy && <SkeletonRows n={4} h={104} />}
          </View>
        }
        ListEmptyComponent={loaded && !err ? <EmptyState icon="docCheck" title={q ? "Nothing matches" : "No documents issued yet"} text={q ? "Check the reference, or verify it directly." : "Issued documents appear here."} /> : undefined}
        renderItem={({ item: d, index }) => {
          const valid = d.status === "valid";
          return (
            <FadeIn index={index}>
              <PressCard rail={valid ? c.ok : c.bad} onPress={() => void verify(d.ref)} label={`Verify ${d.title}`}>
                <View style={{ flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 }}>
                  <Text style={[s.mono, { color: c.goldText, fontSize: 12.5 }]} selectable numberOfLines={1}>{d.ref}</Text>
                  <Pill text={d.status} tone={valid ? "ok" : "bad"} icon={valid ? "check" : "x"} />
                </View>
                <Text style={[s.text, s.semibold, { fontSize: 16 }]}>{d.title}</Text>
                <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
                  <Text style={[s.mono, { color: c.muted, fontSize: 11 }]}>{d.code} · {day(d.issued_at)}</Text>
                  <View style={{ flexDirection: "row", alignItems: "center", gap: 6 }}>
                    <Icon name="scan" size={15} color={c.goldText} />
                    <Text style={{ color: c.goldText, fontSize: 13, fontFamily: font.semibold }}>{checking === d.ref ? "Checking…" : "Verify"}</Text>
                  </View>
                </View>
              </PressCard>
            </FadeIn>
          );
        }}
      />
    </Screen>
  );
}
