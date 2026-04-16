export interface ApiEnvelope<T> {
  data?: T;
  success?: boolean;
  message?: string;
  error?: string;
}
