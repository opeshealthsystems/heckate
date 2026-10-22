import { useEffect, useRef, useState, type ReactNode } from "react";
import { Animated, Easing, type ViewStyle } from "react-native";
import { useReduceMotion } from "./theme";

/** Re-renders every `ms` milliseconds with the current time (live SLA timers). Pass 0 to stop. */
export function useNow(ms = 1000): number {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!ms) return;
    setNow(Date.now());
    const id = setInterval(() => setNow(Date.now()), ms);
    return () => clearInterval(id);
  }, [ms]);
  return now;
}

/** Fades and lifts its children in; `index` staggers a list. With reduce-motion on it appears at once. */
export function FadeIn({ children, index = 0, style }: { children: ReactNode; index?: number; style?: ViewStyle }) {
  const reduce = useReduceMotion();
  const v = useRef(new Animated.Value(reduce ? 1 : 0)).current;
  useEffect(() => {
    if (reduce) { v.setValue(1); return; }
    Animated.timing(v, { toValue: 1, duration: 320, delay: Math.min(index, 8) * 45, easing: Easing.out(Easing.cubic), useNativeDriver: true }).start();
  }, [reduce, index, v]);
  return (
    <Animated.View style={[style, { opacity: v, transform: [{ translateY: v.interpolate({ inputRange: [0, 1], outputRange: [10, 0] }) }] }]}>
      {children}
    </Animated.View>
  );
}

/** A 0..1 value that eases toward `to` (ring arcs, bars). Instant with reduce-motion. */
export function useEased(to: number, ms = 700): Animated.Value {
  const reduce = useReduceMotion();
  const v = useRef(new Animated.Value(reduce ? to : 0)).current;
  useEffect(() => {
    if (reduce) { v.setValue(to); return; }
    Animated.timing(v, { toValue: to, duration: ms, easing: Easing.out(Easing.cubic), useNativeDriver: false }).start();
  }, [to, ms, reduce, v]);
  return v;
}
