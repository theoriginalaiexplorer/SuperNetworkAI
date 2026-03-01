// Alpine.js type declarations
// Add type safety for Alpine.js global object
declare const Alpine: {
  data(name: string, dataFn: () => any): any;
  store(name: string, dataFn: () => any): any;
  directive(name: string, directiveFn: (el: any, expression: any) => void): void;
};
