import { Image, Text } from "react-native";
import { useTheme } from "../lib/theme";

const WHITE = require("../../assets/seal-white.png");
const NAVY = require("../../assets/seal-navy.png");

/**
 * The round HEKATE platform seal, whole and uncropped: white on dark surfaces, navy on light ones.
 * `onDark` forces the white seal on brand surfaces that are always dark (sign-in, request access).
 */
export function Seal({ size = 96, onDark }: { size?: number; onDark?: boolean }) {
  const { scheme } = useTheme();
  return <Image source={onDark || scheme === "dark" ? WHITE : NAVY} style={{ width: size, height: size }} resizeMode="contain" accessibilityLabel="HEKATE seal" />;
}

/**
 * The HEKATE wordmark. The ONLY use of the brand display face (Cinzel); every other string in the app is the system font.
 * Deliberate deviation from the brand pack, flagged for the owner's sign-off in docs/MOBILE_DESIGN.md.
 */
export function Wordmark({ size = 30 }: { size?: number }) {
  const { c } = useTheme();
  return (
    <Text accessibilityRole="header" maxFontSizeMultiplier={1.2} style={{ fontFamily: "Cinzel_700Bold", fontSize: size, lineHeight: size * 1.25, letterSpacing: size * 0.12, color: c.label as string }}>
      HEKATE
    </Text>
  );
}
