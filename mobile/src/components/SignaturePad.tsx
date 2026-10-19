import { useRef, useState, type ComponentRef } from "react";
import { PanResponder, Text, View } from "react-native";
import Svg, { Path } from "react-native-svg";
import { captureRef } from "react-native-view-shot";
import { Button } from "./ui";
import { useTheme } from "../lib/theme";

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
      <View ref={box} collapsable={false} style={{ height: 160, backgroundColor: "#ffffff", borderColor: c.line, borderWidth: 1, borderRadius: 8 }} {...pan.panHandlers}>
        <Svg width="100%" height="100%">
          {[...paths, current].filter(Boolean).map((d, i) => (
            <Path key={i} d={d} stroke="#111111" strokeWidth={2.5} fill="none" strokeLinecap="round" strokeLinejoin="round" />
          ))}
        </Svg>
      </View>
      <View style={s.row}>
        <Button title="Clear" onPress={clear} />
        <Button primary title={saved ? "Signature saved" : "Use this signature"} disabled={paths.length === 0} onPress={() => void done()} />
      </View>
      {saved && <Text style={s.muted}>Signature captured.</Text>}
    </View>
  );
}
