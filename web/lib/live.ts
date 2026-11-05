"use client";
import { useEffect, useRef } from "react";
import { API_URL } from "./api";
import { getToken } from "./auth";

export interface LiveEvent { subject: string; data: { id?: string; tenant_id?: string } & Record<string, unknown> }

/** WebSocket address for the API, with the scheme swapped. */
export const streamUrl = () => `${API_URL.replace(/^http/, "ws")}/v1/stream`;

/** The bearer token travels as a subprotocol: browsers cannot set headers on a WebSocket and a query string would be logged. */
export const streamProtocols = (token: string) => ["hekate.v1", `bearer.${token}`];

/**
 * Calls onEvent for each live event, reconnecting with a growing delay if the connection drops.
 * Events are hints to refetch: the data itself still comes from the API.
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
      ws = new WebSocket(streamUrl(), streamProtocols(token));
      ws.onopen = () => { delay = 1000; };
      ws.onmessage = (m) => {
        try { handler.current(JSON.parse(m.data as string)); } catch { /* ignore malformed */ }
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
