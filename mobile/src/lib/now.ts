import { useEffect, useState } from "react";

/** Re-renders every `ms` milliseconds with the current time (SLA lines). Lists use 30 s. Pass 0 to stop. */
export function useNow(ms = 30_000): number {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!ms) return;
    setNow(Date.now());
    const id = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(id);
  }, [ms]);
  return now;
}
