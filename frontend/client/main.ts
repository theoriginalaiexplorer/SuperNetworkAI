// Alpine.js stores are registered here in later phases.
// This file is the Vite entry point for all client-side assets.

import "htmx.org";
import Alpine from "alpinejs";
import { loadingStore } from "./stores/loading";

// Make Alpine globally available for inline scripts
(window as any).Alpine = Alpine;

Alpine.store("loading", loadingStore);
Alpine.start();
