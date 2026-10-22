import Svg, { Path } from "react-native-svg";

/** Line icons on a 24-pt grid, one stroke weight. Decorative: the visible label carries the meaning. */
const PATHS = {
  // navigation (the first six are the same paths as the web ERP menu)
  tickets: "M3 9a2 2 0 0 0 0 6v3a1 1 0 0 0 1 1h16a1 1 0 0 0 1-1v-3a2 2 0 0 1 0-6V6a1 1 0 0 0-1-1H4a1 1 0 0 0-1 1zM13 5v14",
  field: "M12 21s-7-6.2-7-11a7 7 0 0 1 14 0c0 4.8-7 11-7 11zM12 12a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5z",
  people: "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8",
  home: "M3 11l9-8 9 8M5 10v10h5v-6h4v6h5V10",
  documents: "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8zM14 3v5h5M9 13h6M9 17h6",
  plus: "M12 5v14M5 12h14",
  // security
  shield: "M12 3l8 3v6c0 4.5-3.2 8.3-8 9-4.8-.7-8-4.5-8-9V6z",
  shieldCheck: "M12 3l8 3v6c0 4.5-3.2 8.3-8 9-4.8-.7-8-4.5-8-9V6zM8.5 12l2.5 2.5 4.5-5",
  lock: "M6 11h12v9H6zM8.5 11V8a3.5 3.5 0 0 1 7 0v3M12 15v1.5",
  unlock: "M6 11h12v9H6zM8.5 11V8a3.5 3.5 0 0 1 6.8-1.2M12 15v1.5",
  key: "M8 15a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM11 12h10M18 12v3M15 12v2",
  fingerprint: "M5 8a9 9 0 0 1 14 0M7.5 11.5a5 5 0 0 1 9 0M12 11a3 3 0 0 0-3 3c0 2 .3 3.8 1 5.5M12 14c0 2.2.4 4.2 1.3 6M15 14.5c0 2 .3 3.5.8 5M6.5 14c0 1.7.2 3 .6 4.4M17.5 17.5c.3 1 .6 1.7 1 2.5",
  radar: "M12 3a9 9 0 1 0 9 9M12 7.5a4.5 4.5 0 1 0 4.5 4.5M12 12l6.5-6.5M12 12h.01",
  pulse: "M3 12h4l2.5-6 4 12 2.5-6H21",
  alert: "M12 3.5L21.5 20h-19zM12 10v4.5M12 17.5h.01",
  // content
  docCheck: "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8zM14 3v5h5M9 14.5l2 2 4-4",
  docX: "M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8zM14 3v5h5M10 13.5l4 4M14 13.5l-4 4",
  wrench: "M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.8-3.8a6 6 0 0 1-7.9 7.9l-6.9 6.9a2.1 2.1 0 0 1-3-3l6.9-6.9a6 6 0 0 1 7.9-7.9l-3.8 3.8z",
  scan: "M4 8V4h4M16 4h4v4M20 16v4h-4M8 20H4v-4M8 12h8",
  clock: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 7v5l3 2",
  calendar: "M4 6h16v14H4zM4 10h16M8 3v4M16 3v4",
  pen: "M4 20l4-1 11-11-3-3L5 16zM14 6l3 3",
  camera: "M4 8h3l2-3h6l2 3h3v11H4zM12 17a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z",
  image: "M4 5h16v14H4zM4 16l5-5 4 4 3-3 4 4M9 9.5h.01",
  user: "M20 21v-1a5 5 0 0 0-5-5H9a5 5 0 0 0-5 5v1M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z",
  send: "M21 3L10 14M21 3l-7 18-4-7-7-4z",
  // system
  sync: "M21 12a9 9 0 0 1-15.5 6.2M3 12A9 9 0 0 1 18.5 5.8M18.5 2v4h-4M5.5 22v-4h4",
  wifi: "M3 9.5a14 14 0 0 1 18 0M6 13a9.5 9.5 0 0 1 12 0M9 16.5a5 5 0 0 1 6 0M12 20h.01",
  wifiOff: "M3 3l18 18M9 16.5a5 5 0 0 1 4.5-1.2M6 13a9.5 9.5 0 0 1 3-1.8M3 9.5a14 14 0 0 1 4-2.4M12 20h.01M15.5 9.2A14 14 0 0 1 21 9.5M17.5 12.4c.2.2.4.4.5.6",
  chevron: "M9 6l6 6-6 6",
  back: "M15 6l-6 6 6 6",
  check: "M5 12.5l4.5 4.5L19 7.5",
  x: "M6 6l12 12M18 6L6 18",
  search: "M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM21 21l-4.5-4.5",
  logout: "M9 4H5v16h4M16 8l4 4-4 4M20 12H9",
  moon: "M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z",
  sun: "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8zM12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5L19 19M5 19l1.5-1.5M17.5 6.5L19 5",
  info: "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM12 11v5M12 8h.01",
  inbox: "M3 13l3-8h12l3 8v6H3zM3 13h5l1 3h6l1-3h5",
} as const;
export type IconName = keyof typeof PATHS;

export function Icon({ name, color, size = 24, stroke = 1.7 }: { name: IconName; color: string; size?: number; stroke?: number }) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth={stroke} strokeLinecap="round" strokeLinejoin="round" accessibilityElementsHidden importantForAccessibility="no-hide-descendants">
      <Path d={PATHS[name]} />
    </Svg>
  );
}
