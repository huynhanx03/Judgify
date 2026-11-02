import { REALTIME_API } from "@/constants/api/realtime";
import { api } from "@/lib/api/client";
import { isoDateTimeSchema } from "@/lib/api/contracts";
import {
  strictObjectSchema,
  stringSchema,
} from "@/lib/api/schema";

interface RealtimeTicketResponse {
  ticket: string;
  expires_at: string;
}

const realtimeTicketResponseSchema = strictObjectSchema({
  ticket: stringSchema({
    minimumLength: 43,
    maximumLength: 43,
    pattern: /^[A-Za-z0-9_-]{43}$/,
    label: "realtime ticket",
  }),
  expires_at: isoDateTimeSchema,
});

/** Issues one short-lived credential per WebSocket upgrade; never cache or retry it. */
export const realtimeService = {
  async issueTicket(signal?: AbortSignal): Promise<string> {
    const response = await api<RealtimeTicketResponse, never>(
      REALTIME_API.TICKET,
      {
        method: "POST",
        signal,
        retryUnauthorized: false,
        schema: realtimeTicketResponseSchema,
      },
    );
    return response.ticket;
  },
};
