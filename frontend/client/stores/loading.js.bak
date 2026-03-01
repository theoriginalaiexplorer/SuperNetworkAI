// Global loading state management for the application
document.addEventListener('alpine:init', () => {
  const app = document.querySelector('[x-data]');

  Alpine.data('loadingStore', () => ({
    isLoading: false,
    message: '',
    startLoading(message) {
      this.isLoading = true;
      this.message = message || 'Loading...';
    },
    stopLoading() {
      this.isLoading = false;
      this.message = '';
    },
  }));

  console.log('Loading store initialized');
});
