import Svg, { Circle, Ellipse, G, Path, Polygon, Rect, Text as SvgText } from "react-native-svg";

/**
 * HS-MOTION-001 "Network Convergence": a 2.8 s scene drawn as a pure function of time t (seconds).
 * Internet, router, server, cloud and backup connect, then their traces converge into the HEKATE shield.
 */
export const CONVERGENCE_END = 2.8;

const STEEL = "#9AA7B5", FLOW = "#67B8F3", LINK = "#3A5F8A", BG = "#071D38";
const ease = (x: number) => (x <= 0 ? 0 : x >= 1 ? 1 : x < 0.5 ? 2 * x * x : 1 - Math.pow(-2 * x + 2, 2) / 2);
const p = (t: number, a: number, b: number) => ease((t - a) / (b - a));

type Pt = [number, number];
// Links as polylines so packets can follow them without measuring SVG paths at runtime.
const LINKS: Record<string, Pt[]> = {
  pL: [[0, 470], [40, 470]],
  pR: [[360, 470], [320, 470]],
  rs: [[88, 470], [276, 470]],
  gr: [[64, 174], [64, 458]],
  sc: [[296, 440], [296, 164]],
  sb: [[296, 500], [296, 610], [202, 610]],
};
const len = (pts: Pt[]) => pts.slice(1).reduce((s, q, i) => s + Math.hypot(q[0] - pts[i][0], q[1] - pts[i][1]), 0);
const at = (pts: Pt[], u: number): Pt => {
  let d = u * len(pts);
  for (let i = 1; i < pts.length; i++) {
    const [x0, y0] = pts[i - 1], [x1, y1] = pts[i], l = Math.hypot(x1 - x0, y1 - y0);
    if (d <= l) return [x0 + ((x1 - x0) * d) / l, y0 + ((y1 - y0) * d) / l];
    d -= l;
  }
  return pts[pts.length - 1];
};
const d = (pts: Pt[]) => "M" + pts.map((q) => q.join(",")).join(" L");

// [link, start, duration, reverse]
const PACKETS: [string, number, number, boolean?][] = [
  ["rs", 0.7, 0.55], ["rs", 0.95, 0.55], ["gr", 1.2, 0.45], ["rs", 1.45, 0.45],
  ["sc", 1.3, 0.5], ["sc", 1.55, 0.45], ["sb", 1.6, 0.35], ["sb", 1.75, 0.3, true], ["sb", 1.85, 0.35],
];

// Convergence traces: [path, approx length, start]
const TRACES: [string, number, number][] = [
  ["M86,150 C130,150 150,240 168,290", 175, 1.95],
  ["M274,150 C230,150 210,240 192,290", 175, 1.95],
  ["M90,470 C130,470 150,400 168,350", 145, 2.0],
  ["M272,470 C230,470 210,400 192,350", 145, 2.0],
  ["M180,590 V490", 100, 2.05],
];

/** Stroke that draws itself in between a and b. */
const draw = (t: number, a: number, b: number, L: number) => ({ strokeDasharray: [L, L], strokeDashoffset: L * (1 - p(t, a, b)) });

/** `ambient`: the connected network with packets looping forever and no brand resolution (sign-in backdrop). */
export function ConvergenceScene({ t: clock, ambient }: { t: number; ambient?: boolean }) {
  const t = ambient ? 1.9 : clock;
  const pt = ambient ? 0.7 + (clock % 1.5) : clock;
  const ic = { fill: "none", stroke: STEEL, strokeWidth: 1.75, strokeLinecap: "round" as const, strokeLinejoin: "round" as const };
  const link = (k: string, a: number, b: number) => {
    const pts = LINKS[k];
    return <Path d={d(pts)} fill="none" stroke={LINK} strokeWidth={1.5} strokeLinecap="round" {...draw(t, a, b, len(pts))} />;
  };
  const traceFade = t < 2.4 ? 1 : 1 - p(t, 2.4, 2.7);
  const emb = p(t, 2.0, 2.4);

  return (
    <Svg width="100%" height="100%" viewBox="0 0 360 800" preserveAspectRatio="xMidYMid meet">
      {/* 01 ignition */}
      {link("pL", 0, 0.35)}
      {link("pR", 0, 0.35)}
      <Circle cx={40} cy={470} r={2.5} fill={FLOW} opacity={p(t, 0.2, 0.35)} />
      <Circle cx={320} cy={470} r={2.5} fill={FLOW} opacity={p(t, 0.2, 0.35)} />

      {/* 02 router + server */}
      <G x={64} y={470}>
        <Rect x={-24} y={-12} width={48} height={24} rx={4} {...ic} {...draw(t, 0.35, 0.7, 140)} />
        <G opacity={p(t, 0.6, 0.75)}>
          {[-15, -7, 1].map((x) => <Circle key={x} cx={x} cy={0} r={1.8} fill={FLOW} />)}
          <Path d="M10,-5 v10 M16,-5 v10" {...ic} />
        </G>
      </G>
      <G x={296} y={470}>
        <Rect x={-20} y={-30} width={40} height={60} rx={3} {...ic} {...draw(t, 0.4, 0.75, 196)} />
        <G opacity={p(t, 0.55, 0.85)}>
          <Path d="M-20,-10 H20 M-20,10 H20 M-12,-20 h8 M-12,0 h8 M-12,20 h8" {...ic} />
          {[-20, 0, 20].map((y) => <Circle key={y} cx={12} cy={y} r={1.6} fill={FLOW} />)}
        </G>
      </G>
      {link("rs", 0.55, 0.85)}
      <G x={180} y={470} opacity={p(t, 0.85, 1.1)}>
        <Path d="M-9,-11 L0,-15 L9,-11 V-2 C9,6 4,11 0,13 C-4,11 -9,6 -9,-2 Z" {...ic} fill={BG} />
      </G>

      {/* 03 internet + cloud */}
      <G x={64} y={150}>
        <Circle r={22} {...ic} {...draw(t, 0.85, 1.15, 139)} />
        <Path d="M-22,0 H22 M0,-22 C-11,-10 -11,10 0,22 M0,-22 C11,-10 11,10 0,22 M-18,-12 H18 M-18,12 H18" {...ic} opacity={p(t, 1, 1.3)} />
      </G>
      <G x={296} y={150}>
        <Path d="M-22,10 H20 A11,11 0 0 0 18,-11 A15,15 0 0 0 -10,-12 A12,12 0 0 0 -22,10 Z" {...ic} {...draw(t, 1.05, 1.4, 150)} />
      </G>
      {link("gr", 0.95, 1.25)}
      {link("sc", 1.1, 1.4)}

      {/* 04 backup */}
      <G x={180} y={610} opacity={p(t, 1.4, 1.7)}>
        <Ellipse cx={0} cy={-14} rx={20} ry={6} {...ic} />
        <Path d="M-20,-14 V14 C-20,22 20,22 20,14 V-14 M-20,0 C-20,8 20,8 20,0" {...ic} />
      </G>
      {link("sb", 1.4, 1.6)}

      {/* packets */}
      {PACKETS.map(([k, s, dur, rev], i) => {
        const u = (pt - s) / dur;
        if (u < 0 || u > 1) return null;
        const e = ease(u);
        const [x, y] = at(LINKS[k], rev ? 1 - e : e);
        return <Circle key={i} cx={x} cy={y} r={3} fill={FLOW} opacity={Math.min(1, Math.sin(Math.PI * u) * 2)} />;
      })}

      {/* 05 convergence */}
      {!ambient && (<>
      <G opacity={traceFade}>
        {TRACES.map(([path, L, s], i) => (
          <Path key={i} d={path} fill="none" stroke={FLOW} strokeWidth={1.25} strokeLinecap="round" {...draw(t, s, s + 0.35, L)} />
        ))}
      </G>
      <G x={180} y={310} scale={0.15 + 0.015 * emb} opacity={emb}>
        <G x={-627} y={-628}>
          <Path d="M627,148 C800,160 950,205 1055,272 L1056,560 C1056,800 900,980 627,1110 C354,980 198,800 198,560 L199,272 C304,205 454,160 627,148 Z" fill={BG} stroke="#fff" strokeWidth={40} strokeLinejoin="round" />
          <Path opacity={p(t, 2.2, 2.4)} d="M262,368 C420,260 540,245 627,245 C714,245 834,260 992,370 C834,305 714,288 627,288 C540,288 420,305 262,368 Z" fill="#00B8FF" />
          <Path opacity={p(t, 2.15, 2.35)} d="M363,378 H484 a6,6 0 0 1 6,6 V510 H765 V384 a6,6 0 0 1 6,-6 H891 a6,6 0 0 1 6,6 V756 a6,6 0 0 1 -6,6 H771 a6,6 0 0 1 -6,-6 V597 H490 V756 a6,6 0 0 1 -6,6 H363 a6,6 0 0 1 -6,-6 V384 a6,6 0 0 1 6,-6 Z" fill="#fff" />
          <Polygon opacity={p(t, 2.25, 2.4)} points="525,766 627,621 730,766" fill="#00B8FF" />
          <G stroke="#fff" strokeWidth={11} fill="none" opacity={p(t, 2.2, 2.4)}>
            <Path d="M627,1085 V838" /><Path d="M572,1058 V925 L507,862" /><Path d="M685,1058 V925 L750,862" />
            <Circle cx={627} cy={818} r={16} /><Circle cx={494} cy={850} r={16} /><Circle cx={760} cy={850} r={16} />
          </G>
        </G>
      </G>

      </>)}
      {/* 06 brand */}
      {!ambient && <SvgText x={180} y={712} textAnchor="middle" fill="#fff" fontSize={22} fontFamily="Montserrat_700Bold" opacity={p(t, 2.4, 2.65)}>HEKATE SYSTEMS SARL</SvgText>}
      {!ambient && <SvgText x={180} y={738} textAnchor="middle" fill={STEEL} fontSize={11.5} fontFamily="Montserrat_500Medium" opacity={p(t, 2.5, 2.8)}>Engineering Trust. Securing Tomorrow.</SvgText>}
    </Svg>
  );
}
