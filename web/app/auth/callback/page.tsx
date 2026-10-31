"use client";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { finishLogin } from "@/lib/auth";

export default function Callback() {
  const router = useRouter();
  const [err, setErr] = useState("");
  useEffect(() => {
    finishLogin(new URLSearchParams(location.search)).then(() => router.replace("/dashboard")).catch((e) => setErr(e.message));
  }, [router]);
  return <main className="center">{err ? <p className="error">{err}</p> : <p className="muted">Signing you in…</p>}</main>;
}
