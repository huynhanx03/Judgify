/** Realtime endpoints. Ticket issuance is authenticated; the WebSocket is not. */
export const REALTIME_API = {
  TICKET: "/realtime/tickets",
  WEBSOCKET: "/ws",
} as const;
