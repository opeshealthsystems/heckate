import { xaf } from "@/lib/format";

export function Totals({ net, tva, cac, total }: { net: number; tva: number; cac: number; total: number }) {
  return (
    <div className="totals">
      <div><span>Net</span><span className="num">{xaf(net)}</span></div>
      <div><span>TVA</span><span className="num">{xaf(tva)}</span></div>
      <div><span>CAC</span><span className="num">{xaf(cac)}</span></div>
      <div className="grand"><span>Total</span><span className="num">{xaf(total)}</span></div>
    </div>
  );
}
