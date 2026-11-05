import { useEffect, useRef, useState } from "react";
import { AccessibilityInfo } from "react-native";

/** The elapsed-time clock (about 30 fps). Freezes on the finished scene when the person has asked for reduced motion. */
export function useClock(start = 0) {
  const [t, setT] = useState(start);
  const reduce = useRef(false);
  useEffect(() => {
    let raf = 0, alive = true, last = 0;
    const t0 = Date.now() - start * 1000;
    void AccessibilityInfo.isReduceMotionEnabled().then((r) => {
      reduce.current = r;
      if (r) setT(9);
    });
    const tick = () => {
      if (!alive) return;
      const now = Date.now();
      if (!reduce.current && now - last > 33) {
        last = now;
        setT((now - t0) / 1000);
      }
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => {
      alive = false;
      cancelAnimationFrame(raf);
    };
  }, [start]);
  return t;
}
