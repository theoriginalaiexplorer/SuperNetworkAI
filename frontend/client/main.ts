// Alpine.js stores are registered here in later phases.
// This file is the Vite entry point for all client-side assets.

import Alpine from "alpinejs";
import { chatStore } from "./stores/chat";
import { loadingStore } from "./stores/loading";

// Alpine.store("chat", chatStore);
Alpine.store("loading", loadingStore);
Alpine.start();
