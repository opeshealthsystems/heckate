import { useRef, useState, type ComponentRef } from "react";
import { PanResponder, Text, View } from "react-native";
import Svg, { Line, Path } from "react-native-svg";
import { captureRef } from "react-native-view-shot";
import { haptic } from "../lib/haptics";
import { radius, useTheme } from "../lib/theme";
import { Button } from "./ui";

// The pad is paper: white with dark ink in every theme, because the PNG is stored on the acceptance document.
const INK = "#0B1B33";

/** Draw a signature with a finger. `onDone` receives a PNG data URL, or null after Clear. */
export function SignaturePad({ onDone }: { onDone: (png: string | null) => void }) {
  const { s, c } = useTheme();
  const box = useRef<ComponentRef<typeof View>>(null);
  const [paths, setPaths] = useState<string[]>([]);
  const [current, setCurrent] = useState("");
  const live = useRef("");
  const finished = useRef<string[]>([]);
  const [saved, setSaved] = useState(false);

  const pan = useRef(
    PanResponder.create({
      onStartShouldSetPanResponder: () => true,
      onMoveShouldSetPanResponder: () => true,
      onPanResponderTerminationRequest: () => false,
      onPanResponderGrant: (e) => {
        live.current = `M${e.nativeEvent.locationX.toFixed(1)},${e.nativeEvent.locationY.toFixed(1)}`;
        setCurrent(live.current);
        setSaved(false);
      },
      onPanResponderMove: (e) => {
        live.current += ` L${e.nativeEvent.locationX.toFixed(1)},${e.nativeEvent.locationY.toFixed(1)}`;
        setCurrent(live.current);
      },
      onPanResponderRelease: () => {
        finished.current = [...finished.current, live.current];
        setPaths(finished.current);
        live.current = "";
        setCurrent("");
      },
    }),
  ).current;

  async function done() {
    if (!box.current || paths.length === 0) return;
    const uri = await captureRef(box.current, { format: "png", result: "data-uri", quality: 1 });
    haptic.success();
    setSaved(true);
    onDone(uri);
  }
  function clear() {
    finished.current = [];
    setPaths([]);
    setCurrent("");
    setSaved(false);
    onDone(null);
  }

  return (
    <View style={{ gap: 8 }}>
      <View ref={box} collapsable={false} accessibilityLabel="Signature area" style={{ height: 170, backgroundColor: "#ffffff", borderColor: saved ? c.ok : c.gold, borderWidth: 1.5, borderRadius: radius.md, overflow: "hidden" }} {...pan.panHandlers}>
        <Svg width="100%" height="100%">
          <Line x1="16" x2="96%" y1="132" y2="132" stroke="#9FB3CC" strokeWidth={1} strokeDasharray="4 4" />
          {[...paths, current].filter(Boolean).map((d, i) => (
            <Path key={i} d={d} stroke={INK} strokeWidth={2.6} fill="none" strokeLinecap="round" strokeLinejoin="round" />
          ))}
        </Svg>
        {paths.length === 0 && !current && <Text pointerEvents="none" style={{ position: "absolute", left: 16, top: 138, color: "#5B6675", fontSize: 12, letterSpacing: 1.2, textTransform: "uppercase" }}>Sign above the line</Text>}
      </View>
      <View style={{ flexDirection: "row", gap: 8 }}>
        <Button small icon="x" title="Clear" onPress={clear} style={{ flex: 1 }} />
        <Button small primary icon={saved ? "check" : "pen"} title={saved ? "Signature saved" : "Use signature"} disabled={paths.length === 0} onPress={() => void done()} style={{ flex: 2 }} />
      </View>
      {saved && <Text style={s.muted}>Signature captured. It is attached to the acceptance document when you complete the visit.</Text>}
    </View>
  );
}
