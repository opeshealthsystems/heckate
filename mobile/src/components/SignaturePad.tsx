import { useRef, useState, type ComponentRef } from "react";
import { Modal, PanResponder, Platform, StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import Svg, { Line, Path } from "react-native-svg";
import { captureRef } from "react-native-view-shot";
import { haptic } from "../lib/haptics";
import { radius, sp, useTheme } from "../lib/theme";
import { Button } from "./Button";
import { Text } from "./Text";

// The pad is paper: white with dark ink in every appearance, because the PNG is stored on the acceptance document.
const PAPER = "#FFFFFF";
const INK = "#0B1B33";
const GUIDE = "#8E8E93";

/**
 * A native modal with a paper-white signature area. `onDone` receives a PNG data URL when "Use signature" is tapped.
 * Cancel closes without changing anything.
 */
export function SignatureSheet({ visible, onClose, onDone }: { visible: boolean; onClose: () => void; onDone: (png: string) => void }) {
  const { c } = useTheme();
  const insets = useSafeAreaInsets();
  const box = useRef<ComponentRef<typeof View>>(null);
  const [paths, setPaths] = useState<string[]>([]);
  const [current, setCurrent] = useState("");
  const live = useRef("");
  const finished = useRef<string[]>([]);

  const pan = useRef(
    PanResponder.create({
      onStartShouldSetPanResponder: () => true,
      onMoveShouldSetPanResponder: () => true,
      onPanResponderTerminationRequest: () => false,
      onPanResponderGrant: (e) => {
        live.current = `M${e.nativeEvent.locationX.toFixed(1)},${e.nativeEvent.locationY.toFixed(1)}`;
        setCurrent(live.current);
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

  async function use() {
    if (!box.current || paths.length === 0) return;
    const uri = await captureRef(box.current, { format: "png", result: "data-uri", quality: 1 });
    haptic.success();
    onDone(uri);
    onClose();
  }
  function clear() {
    finished.current = [];
    setPaths([]);
    setCurrent("");
  }

  return (
    <Modal visible={visible} animationType="slide" presentationStyle="pageSheet" onRequestClose={onClose}>
      <View style={{ flex: 1, backgroundColor: c.bg, paddingTop: Platform.OS === "ios" ? sp.lg : insets.top + sp.sm, paddingBottom: insets.bottom + sp.lg, paddingHorizontal: sp.lg, gap: sp.lg }}>
        <View style={{ flexDirection: "row", alignItems: "center", justifyContent: "space-between" }}>
          <Button title="Cancel" onPress={onClose} style={{ paddingHorizontal: 0 }} />
          <Text v="headline" weight="semibold" accessibilityRole="header">Signature</Text>
          <Button title="Clear" onPress={clear} disabled={paths.length === 0} style={{ paddingHorizontal: 0 }} />
        </View>
        <View ref={box} collapsable={false} accessibilityLabel="Signature area" style={{ height: 240, backgroundColor: PAPER, borderRadius: radius.group, borderWidth: StyleSheet.hairlineWidth, borderColor: c.separator, overflow: "hidden" }} {...pan.panHandlers}>
          <Svg width="100%" height="100%">
            <Line x1="16" x2="97%" y1="190" y2="190" stroke={GUIDE} strokeWidth={1} />
            {[...paths, current].filter(Boolean).map((d, i) => (
              <Path key={i} d={d} stroke={INK} strokeWidth={2.6} fill="none" strokeLinecap="round" strokeLinejoin="round" />
            ))}
          </Svg>
          {paths.length === 0 && !current && <Text v="footnote" pointerEvents="none" style={{ position: "absolute", left: 16, top: 196, color: GUIDE }}>Sign above the line</Text>}
        </View>
        <Text v="footnote" tone="label2">The signature is attached to the acceptance document when you complete the visit.</Text>
        <Button kind="primary" title="Use signature" disabled={paths.length === 0} onPress={() => void use()} />
      </View>
    </Modal>
  );
}
