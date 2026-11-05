import { useEffect, useRef, useState } from "react";
import { Animated, Pressable, StyleSheet } from "react-native";
import { ConvergenceScene, CONVERGENCE_END } from "./ConvergenceSplash";
import { useClock } from "../lib/clock";
import { PerimeterScene } from "./PerimeterSplash";

// Two launch animations; each cold start picks one so the opening stays fresh.
const SCENES = [ConvergenceScene, PerimeterScene];

// The 2.8 s convergence plus a short hold on the resolved brand.
const SHOW_FOR_MS = 3400;

/**
 * The animated launch screen. Takes over from the native (static emblem) splash on every cold start,
 * plays the network bring-up, then fades into the app. A tap skips it.
 */
export function AnimatedSplash() {
  const [done, setDone] = useState(false);
  const fade = useRef(new Animated.Value(1)).current;
  const t = useClock();
  const [Scene] = useState(() => SCENES[Math.floor(Math.random() * SCENES.length)]);

  const finish = () =>
    Animated.timing(fade, { toValue: 0, duration: 450, useNativeDriver: true }).start(() => setDone(true));

  useEffect(() => {
    const id = setTimeout(finish, SHOW_FOR_MS);
    return () => clearTimeout(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (done) return null;
  return (
    <Animated.View style={[StyleSheet.absoluteFill, { opacity: fade, backgroundColor: "#071D38", zIndex: 90 }]}>
      <Pressable style={StyleSheet.absoluteFill} onPress={finish} accessibilityRole="button" accessibilityLabel="HEKATE is starting. Tap to skip.">
        <Scene t={Math.min(t, CONVERGENCE_END)} />
      </Pressable>
    </Animated.View>
  );
}
