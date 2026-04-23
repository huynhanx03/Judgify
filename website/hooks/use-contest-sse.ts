/**
 * SSE hook for realtime contest standings.
 * Connects to backend EventSource endpoint and auto-reconnects.
 */

import { useEffect, useState, useRef, useCallback } from "react";
import { BASE_API_URL } from "@/constants/api";
import type { Standing } from "@/types/contest";

function getAccessToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("judgify_access_token");
}

interface UseContestSSE {
  standings: Standing[];
  isConnected: boolean;
}

export function useContestSSE(contestId: number | null, enabled: boolean): UseContestSSE {
  const [standings, setStandings] = useState<Standing[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const esRef = useRef<EventSource | null>(null);
  const retryRef = useRef(0);

  const connect = useCallback(() => {
    if (!contestId || !enabled) return;

    // Clean up existing connection
    if (esRef.current) {
      esRef.current.close();
    }

    const token = getAccessToken();
    const url = `${BASE_API_URL}/contests/${contestId}/standings/stream${token ? `?token=${encodeURIComponent(token)}` : ""}`;
    const es = new EventSource(url);
    esRef.current = es;

    es.onopen = () => {
      setIsConnected(true);
      retryRef.current = 0;
    };

    es.addEventListener("snapshot", (e) => {
      try {
        const data = JSON.parse(e.data);
        setStandings(Array.isArray(data) ? data : JSON.parse(data));
      } catch { /* ignore parse errors */ }
    });

    es.addEventListener("update", (e) => {
      try {
        const data = JSON.parse(e.data);
        setStandings(Array.isArray(data) ? data : JSON.parse(data));
      } catch { /* ignore parse errors */ }
    });

    es.onerror = () => {
      setIsConnected(false);
      es.close();
      // Exponential backoff: 1s, 2s, 4s, max 30s
      const delay = Math.min(1000 * Math.pow(2, retryRef.current), 30000);
      retryRef.current++;
      setTimeout(() => {
        if (enabled) connect();
      }, delay);
    };
  }, [contestId, enabled]);

  useEffect(() => {
    if (!enabled || !contestId) {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      setIsConnected(false);
      return;
    }

    connect();

    return () => {
      if (esRef.current) {
        esRef.current.close();
        esRef.current = null;
      }
      setIsConnected(false);
    };
  }, [contestId, enabled, connect]);

  return { standings, isConnected };
}
