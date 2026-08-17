import { AlertCircle, LoaderCircle, RefreshCw } from "lucide-react";
import { Button } from "@gantry/design-system";

export function LoadingState({ label }: { label: string }) {
  return (
    <div className="admin-state">
      <LoaderCircle className="admin-spin" size={18} />
      <span>{label}</span>
    </div>
  );
}

export function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className="admin-error" role="alert">
      <div className="admin-error-content">
        <AlertCircle size={17} style={{ flexShrink: 0 }} />
        <span>{message}</span>
      </div>
      {onRetry ? (
        <Button size="sm" variant="quiet" onClick={onRetry}>
          <RefreshCw size={13} /> Retry
        </Button>
      ) : null}
    </div>
  );
}
