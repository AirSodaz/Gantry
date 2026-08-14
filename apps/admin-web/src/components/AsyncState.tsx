import { LoaderCircle } from 'lucide-react';

export function LoadingState({ label }: { label: string }) {
  return <div className="admin-state"><LoaderCircle className="admin-spin" size={18} /><span>{label}</span></div>;
}

export function ErrorState({ message }: { message: string }) {
  return <div className="admin-error" role="alert">{message}</div>;
}
