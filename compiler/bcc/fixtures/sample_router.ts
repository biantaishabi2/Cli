import { AgentHandler } from './handlers/agent';
import { SessionStore } from '../stores/session';

export function dispatch(request: Request): Response {
  const handler = new AgentHandler();
  return handler.process(request);
}

export async function handleWebSocket(ws: WebSocket): Promise<void> {
  const session = await SessionStore.get(ws.id);
  // process websocket
}

export const VERSION = '1.0.0';
