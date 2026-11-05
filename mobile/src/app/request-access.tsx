import { useRef, useState, type ComponentRef } from "react";
import { router } from "expo-router";
import { TextInput } from "react-native";
import { AuthField, AuthShell } from "../components/AuthShell";
import { Button } from "../components/Button";
import { Text } from "../components/Text";
import { API_URL } from "../lib/api";

// People without an account ask for one here; a manager approves it in the web admin and Keycloak emails a set-password link.
type Key = "name" | "email" | "phone" | "company" | "reason";
const FIELDS: { k: Key; label: string; ph: string; kb?: "email-address" | "phone-pad"; ac?: "name" | "email" | "tel"; max: number }[] = [
  { k: "name", label: "Full name", ph: "Jane Mbarga", ac: "name", max: 200 },
  { k: "email", label: "Work email", ph: "name@company.com", kb: "email-address", ac: "email", max: 254 },
  { k: "phone", label: "Phone (optional)", ph: "+237 6XX XXX XXX", kb: "phone-pad", ac: "tel", max: 40 },
  { k: "company", label: "Organisation", ph: "Company or site name", max: 200 },
  { k: "reason", label: "Role / reason for access", ph: "e.g. Field technician, Douala", max: 1000 },
];

export default function RequestAccess() {
  const [v, setV] = useState<Record<Key, string>>({ name: "", email: "", phone: "", company: "", reason: "" });
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const refs = useRef<(ComponentRef<typeof TextInput> | null)[]>([]);

  const back = () => (router.canGoBack() ? router.back() : router.replace("/login"));

  const submit = async () => {
    if (!v.name.trim() || !/^\S+@\S+\.\S+$/.test(v.email.trim())) {
      setErr("Enter your full name and a valid work email.");
      return;
    }
    setErr("");
    setBusy(true);
    try {
      const res = await fetch(`${API_URL}/v1/public/account-requests`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: v.name.trim(), email: v.email.trim(), phone: v.phone.trim(), company: v.company.trim(), reason: v.reason.trim() }),
      });
      if (res.status === 429) throw new Error("Too many attempts. Please wait a minute and try again.");
      if (!res.ok) {
        const p = await res.json().catch(() => null);
        throw new Error(p?.detail ? `Please check your details: ${p.detail}.` : "The request could not be sent. Please try again.");
      }
      setSent(true);
    } catch (e) {
      setErr(e instanceof TypeError ? "No connection. Check your network and try again." : e instanceof Error ? e.message : String(e));
    }
    setBusy(false);
  };

  if (sent) {
    return (
      <AuthShell title="Request received">
        <Text tone="label2" style={{ textAlign: "center" }}>
          An administrator will review your request. If it is approved, you will get an email at {v.email.trim()} with a link to set your password.
        </Text>
        <Button kind="primary" title="Back to sign in" onPress={back} />
      </AuthShell>
    );
  }

  return (
    <AuthShell title="Request access" subtitle="Accounts are approved by your administrator">
      {FIELDS.map((f, i) => (
        <AuthField
          key={f.k}
          label={f.label}
          inputRef={(r) => { refs.current[i] = r; }}
          value={v[f.k]}
          onChangeText={(x) => { setV({ ...v, [f.k]: x }); setErr(""); }}
          placeholder={f.ph}
          keyboardType={f.kb}
          autoComplete={f.ac}
          autoCapitalize={f.k === "email" ? "none" : "words"}
          autoCorrect={false}
          maxLength={f.max}
          returnKeyType={i < FIELDS.length - 1 ? "next" : "send"}
          onSubmitEditing={() => (i < FIELDS.length - 1 ? refs.current[i + 1]?.focus() : void submit())}
        />
      ))}
      {!!err && <Text v="footnote" tone="red" accessibilityLiveRegion="polite">{err}</Text>}
      <Button kind="primary" title="Request access" loading={busy} onPress={() => void submit()} />
      <Button title="Back to sign in" onPress={back} />
    </AuthShell>
  );
}
