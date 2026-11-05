import { useEffect, useRef } from "react";
import { API_URL } from "./api";
import { getToken } from "./auth";

export interface LiveEvent { subject: string; data: { id?: string; tenant_id?: string } & Record<string, unknown> }

/**
 * Calls onEvent for each live event, reconnecting with a growing delay if the connection drops.
 * Events are hints to refetch: the data itself still comes from the API.
 * The bearer token travels as a subprotocol (headers are not available on every platform's WebSocket).
 */
export function useLive(onEvent: (e: LiveEvent) => void) {
  const handler = useRef(onEvent);
  handler.current = onEvent;
  useEffect(() => {
    let ws: WebSocket | null = null;
    let stopped = false;
    let delay = 1000;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const connect = () => {
      const token = getToken();
      if (!token || stopped) return;
      ws = new WebSocket(`${API_URL.replace(/^http/, "ws")}/v1/stream`, ["hekate.v1", `bearer.${token}`]);
      ws.onopen = () => { delay = 1000; };
      ws.onmessage = (m) => {
        try { handler.current(JSON.parse(String(m.data))); } catch { /* ignore malformed */ }
      };
      ws.onclose = () => {
        if (stopped) return;
        timer = setTimeout(connect, delay);
        delay = Math.min(delay * 2, 30_000);
      };
    };
    connect();
    return () => {
      stopped = true;
      if (timer) clearTimeout(timer);
      ws?.close();
    };
  }, []);
}
