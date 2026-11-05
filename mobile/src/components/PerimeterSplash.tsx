import Svg, { Circle, Defs, G, Line, LinearGradient, Path, Polygon, Stop, Text as SvgText } from "react-native-svg";

/**
 * HS-MOTION-002 "Secure Perimeter": six facilities form a protected ring, a scan verifies it,
 * and the ring locks onto the HEKATE shield. 2.8 s, a pure function of time t (seconds).
 */
export const PERIMETER_END = 2.8;

const STEEL = "#9AA7B5", FLOW = "#67B8F3", LINK = "#3A5F8A", BG = "#071D38";
const ease = (x: number) => (x <= 0 ? 0 : x >= 1 ? 1 : x < 0.5 ? 2 * x * x : 1 - Math.pow(-2 * x + 2, 2) / 2);
const p = (t: number, a: number, b: number) => ease((t - a) / (b - a));

const CX = 180, CY = 300, R = 118, C = 2 * Math.PI * R;
const GLYPH: Record<string, string> = {
  globe: "M-9,0H9M0,-9C-5,-4-5,4 0,9M0,-9C5,-4 5,4 0,9M-8,-4H8M-8,4H8",
  cloud: "M-9,4H8A4.5,4.5 0 0 0 7,-4.5A6,6 0 0 0-4,-5A5,5 0 0 0-9,4Z",
  server: "M-7,-9H7V9H-7ZM-7,-3H7M-7,3H7M-4,-6h3M-4,0h3M-4,6h3",
  db: "M-7,-6A7,2.5 0 0 0 7,-6A7,2.5 0 0 0-7,-6V6A7,2.5 0 0 0 7,6V-6M-7,0A7,2.5 0 0 0 7,0",
  router: "M-9,-4H9V5H-9ZM4,-4V-8M-4,-4V-8",
  site: "M-8,9V-3L0,-9L8,-3V9ZM-3,9V3H3V9",
};
const NODES = ["globe", "cloud", "server", "db", "router", "site"].map((g, i) => {
  const a = -Math.PI / 2 + (i * Math.PI) / 3;
  return { g, a, x: CX + R * Math.cos(a), y: CY + R * Math.sin(a) };
});
// Packets travel clockwise between neighbouring facilities: [from node, start, duration]
const PACKETS: [number, number, number][] = [[0, 0.9, 0.35], [1, 1.0, 0.35], [2, 1.1, 0.35], [3, 1.2, 0.35], [4, 1.3, 0.35], [5, 1.4, 0.35], [0, 1.35, 0.35], [3, 1.5, 0.35]];

export function PerimeterScene({ t }: { t: number }) {
  const ring = p(t, 0, 0.45);
  const lock = p(t, 1.3, 1.95);
  const su = (t - 1.3) / 0.65;
  let sweep = "";
  if (su > 0 && su < 1) {
    const a1 = -Math.PI / 2 + ease(su) * 2 * Math.PI, a0 = a1 - 0.6;
    sweep = `M${CX},${CY}L${CX + R * Math.cos(a0)},${CY + R * Math.sin(a0)}A${R},${R} 0 0 1 ${CX + R * Math.cos(a1)},${CY + R * Math.sin(a1)}Z`;
  }
  const emb = p(t, 1.95, 2.4);
  const spokeFade = t < 2.4 ? 1 : 1 - p(t, 2.4, 2.75);

  return (
    <Svg width="100%" height="100%" viewBox="0 0 360 800" preserveAspectRatio="xMidYMid meet">
      <Defs>
        <LinearGradient id="sw" x1="0" y1="0" x2="1" y2="0">
          <Stop offset="0" stopColor={FLOW} stopOpacity={0} />
          <Stop offset="1" stopColor={FLOW} stopOpacity={0.35} />
        </LinearGradient>
      </Defs>
      <Circle cx={CX} cy={CY} r={R + 34} fill="none" stroke="#173A63" strokeWidth={1} strokeDasharray={[2, 6]} opacity={p(t, 0.2, 0.6) * 0.8} />
      <Circle cx={CX} cy={CY} r={R} fill="none" stroke={LINK} strokeWidth={1.5} strokeDasharray={[C * ring, C]} rotation={-90} originX={CX} originY={CY} />
      <Circle cx={CX} cy={CY} r={R} fill="none" stroke={FLOW} strokeWidth={2} strokeDasharray={[C * lock, C]} rotation={-90} originX={CX} originY={CY} opacity={t < 2.3 ? 1 : 1 - p(t, 2.3, 2.7) * 0.6} />
      {!!sweep && <Path d={sweep} fill="url(#sw)" />}

      <G opacity={spokeFade}>
        {NODES.map((n, i) => {
          const s = 1.9 + i * 0.03, L = R * 0.58;
          return <Line key={i} x1={n.x} y1={n.y} x2={CX + (n.x - CX) * 0.42} y2={CY + (n.y - CY) * 0.42} stroke={FLOW} strokeWidth={1.25} strokeLinecap="round" strokeDasharray={[L * p(t, s, s + 0.3), L]} />;
        })}
      </G>

      {NODES.map((n, i) => {
        const s = 0.3 + i * 0.1, u = p(t, s, s + 0.25);
        return (
          <G key={n.g} x={n.x} y={n.y} scale={0.7 + 0.3 * u} opacity={u}>
            <Circle r={17} fill={BG} stroke={STEEL} strokeWidth={1.5} />
            <Path d={GLYPH[n.g]} fill="none" stroke={STEEL} strokeWidth={1.5} strokeLinecap="round" strokeLinejoin="round" />
            {t > s + 0.3 && <Circle cx={12} cy={-12} r={2.4} fill={FLOW} />}
          </G>
        );
      })}

      {PACKETS.map(([n, s, d], i) => {
        const u = (t - s) / d;
        if (u < 0 || u > 1) return null;
        const a = NODES[n].a + (ease(u) * Math.PI) / 3;
        return <Circle key={i} cx={CX + R * Math.cos(a)} cy={CY + R * Math.sin(a)} r={3} fill={FLOW} opacity={Math.min(1, Math.sin(Math.PI * u) * 2)} />;
      })}

      <G x={CX} y={CY} scale={0.1 + 0.03 * emb} opacity={emb}>
        <G x={-627} y={-628}>
          <Path d="M627,148 C800,160 950,205 1055,272 L1056,560 C1056,800 900,980 627,1110 C354,980 198,800 198,560 L199,272 C304,205 454,160 627,148 Z" fill={BG} stroke="#fff" strokeWidth={40} strokeLinejoin="round" />
          <Path d="M262,368 C420,260 540,245 627,245 C714,245 834,260 992,370 C834,305 714,288 627,288 C540,288 420,305 262,368 Z" fill="#00B8FF" />
          <Path d="M363,378 H484 a6,6 0 0 1 6,6 V510 H765 V384 a6,6 0 0 1 6,-6 H891 a6,6 0 0 1 6,6 V756 a6,6 0 0 1 -6,6 H771 a6,6 0 0 1 -6,-6 V597 H490 V756 a6,6 0 0 1 -6,6 H363 a6,6 0 0 1 -6,-6 V384 a6,6 0 0 1 6,-6 Z" fill="#fff" />
          <Polygon points="525,766 627,621 730,766" fill="#00B8FF" />
          <G stroke="#fff" strokeWidth={11} fill="none">
            <Path d="M627,1085 V838" /><Path d="M572,1058 V925 L507,862" /><Path d="M685,1058 V925 L750,862" />
            <Circle cx={627} cy={818} r={16} /><Circle cx={494} cy={850} r={16} /><Circle cx={760} cy={850} r={16} />
          </G>
        </G>
      </G>

      <SvgText x={180} y={560} textAnchor="middle" fill="#fff" fontSize={22} fontFamily="Montserrat_700Bold" opacity={p(t, 2.4, 2.65)}>HEKATE SYSTEMS SARL</SvgText>
      <SvgText x={180} y={586} textAnchor="middle" fill={STEEL} fontSize={11.5} fontFamily="Montserrat_500Medium" opacity={p(t, 2.5, 2.8)}>Engineering Trust. Securing Tomorrow.</SvgText>
    </Svg>
  );
}
