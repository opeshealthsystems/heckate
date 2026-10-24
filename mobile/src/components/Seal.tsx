import { Image, Text } from "react-native";
import { useTheme } from "../lib/theme";

/** The HEKATE seal: the brand artwork, unmodified, with no glow or backdrop. */
export function Seal({ size = 96 }: { size?: number }) {
  return <Image source={require("../../assets/splash-icon.png")} accessible accessibilityLabel="HEKATE seal" style={{ width: size, height: size }} resizeMode="contain" />;
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
