"use client";
import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import type { Verification } from "@/lib/types";
import { PublicShell, VerificationResult } from "@/components/Verification";

/** Where a scanned QR code lands: the signature is checked first, then the register. */
export default function SignedCode() {
  const { token } = useParams<{ token: string }>();
  const [res, setRes] = useState<Verification | null>(null);
  const [err, setErr] = useState("");
  useEffect(() => { api.verifyQr(token).then(setRes).catch((e) => setErr((e as Error).message)); }, [token]);
  return (
    <PublicShell>
      {err && <p className="error" role="alert">{err}</p>}
      {!res && !err && <p className="muted" role="status">Checking the signature…</p>}
      {res && <VerificationResult v={res} />}
    </PublicShell>
  );
}
