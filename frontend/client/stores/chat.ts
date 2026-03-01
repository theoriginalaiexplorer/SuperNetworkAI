// Chat store for real-time messaging
import Alpine from "alpinejs";

export function chatStore() {
  return Alpine.data("chat", () => ({
    isConnected: false,
    currentConversationId: null as string | null,
    messages: [] as any[],
  }));
}
