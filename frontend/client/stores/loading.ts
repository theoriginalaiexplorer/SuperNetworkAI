// Global loading state store
export function registerLoadingStore(app: any) {
  const loadingStore = {
    isLoading: false,
    message: '',
  };

  app.globalAlpine('loading', () => ({
    isLoading: false,
    startLoading(message?: string) {
      this.isLoading = true;
      this.message = message || 'Loading...';
    },
    stopLoading() {
      this.isLoading = false;
      this.message = '';
    },
  }));

  return loadingStore;
}

// Alpine directive for loading overlay
export function loadingDirective() {
  return (el: { 'x-data': () => ({ loadingStore: () => ({ isLoading: false, message: '' }) });
}
