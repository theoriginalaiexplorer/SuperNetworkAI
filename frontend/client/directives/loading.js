// Loading state directive for Alpine.js
// Usage: x-data="{ loadingStore: () }({ isLoading: false, message: '' }) }"()"

Alpine.directive('loading', (el, { expression }) => {
  const app = document.querySelector('[x-data]');
  if (!app) return;

  const store = Alpine.evaluate(expression, app);

  el._x_isLoading = store.isLoading;
  el._x_message = store.message;
});
