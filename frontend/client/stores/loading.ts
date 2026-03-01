// Loading state management for application

export const loadingStore = {
  isLoading: false,
  message: '',
  startLoading(message: string) {
    this.isLoading = true;
    this.message = message || 'Loading...';
  },
  stopLoading() {
    this.isLoading = false;
    this.message = '';
  },
};
