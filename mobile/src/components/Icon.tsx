import { Platform } from "react-native";
import { MaterialIcons } from "@expo/vector-icons";
import { SymbolView, type SFSymbol } from "expo-symbols";
import { useTheme, type Colors } from "../lib/theme";

// One icon set behind semantic names: SF Symbols on iOS, Material icons elsewhere.
const MAP = {
  home: ["house", "home"],
  tickets: ["ticket", "confirmation-number"],
  field: ["mappin.and.ellipse", "place"],
  documents: ["doc.text", "description"],
  account: ["person.crop.circle", "account-circle"],
  person: ["person", "person"],
  plus: ["plus", "add"],
  chevron: ["chevron.right", "chevron-right"],
  check: ["checkmark", "check"],
  checkCircle: ["checkmark.circle.fill", "check-circle"],
  circle: ["circle", "radio-button-unchecked"],
  close: ["xmark", "close"],
  lock: ["lock", "lock-outline"],
  lockOff: ["lock.open", "lock-open"],
  faceid: ["faceid", "fingerprint"],
  camera: ["camera", "photo-camera"],
  photo: ["photo", "photo-library"],
  send: ["paperplane", "send"],
  warning: ["exclamationmark.triangle", "warning-amber"],
  info: ["info.circle", "info-outline"],
  calendar: ["calendar", "event"],
  clock: ["clock", "schedule"],
  wrench: ["wrench", "build"],
  signature: ["signature", "draw"],
  sync: ["arrow.triangle.2.circlepath", "sync"],
  offline: ["wifi.slash", "wifi-off"],
  search: ["magnifyingglass", "search"],
  signOut: ["rectangle.portrait.and.arrow.right", "logout"],
  appearance: ["circle.lefthalf.filled", "contrast"],
  server: ["server.rack", "dns"],
  receipt: ["receipt", "receipt-long"],
  verified: ["checkmark.seal.fill", "verified"],
  invalid: ["xmark.seal.fill", "gpp-bad"],
  inbox: ["tray", "inbox"],
  money: ["banknote", "payments"],
  note: ["text.bubble", "chat-bubble-outline"],
  location: ["location", "my-location"],
} as const satisfies Record<string, readonly [SFSymbol, keyof typeof MaterialIcons.glyphMap]>;

export type IconName = keyof typeof MAP;

/** 22 pt in lists, 24 in tab bars, tinted by a colour token. Decorative: the visible label carries the meaning. */
export function Icon({ name, size = 22, tone = "label2" }: { name: IconName; size?: number; tone?: keyof Colors }) {
  const { c } = useTheme();
  const [sf, md] = MAP[name];
  if (Platform.OS === "ios") return <SymbolView name={sf} size={size} tintColor={c[tone]} weight="regular" resizeMode="scaleAspectFit" style={{ width: size, height: size }} />;
  return <MaterialIcons name={md} size={size} color={c[tone] as string} />;
}

export const sfName = (n: IconName) => MAP[n][0];
export const mdName = (n: IconName) => MAP[n][1];
