import { AgentHandler } from './handlers/agent';
import { SessionStore } from '../stores/session';

SessionStore.get("bootstrap");

export function dispatch(request: Request): Response {
  return request as unknown as Response;
}

export async function handleWebSocket(ws: WebSocket): Promise<void> {
  void ws;
}

export const VERSION = '1.0.0';
