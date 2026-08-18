import { AlertCircle, LoaderCircle } from "lucide-react";
import { Button } from "@gantry/design-system";

export function LoadingState({ label = "Loading" }: { label?: string }) {
  return (
    <div className="state-block">
      <LoaderCircle className="spin" size={20} aria-hidden="true" />
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
    <div className="state-block state-error">
      <AlertCircle size={20} aria-hidden="true" />
      <div>
        <strong>Something went wrong</strong>
        <p>{message}</p>
      </div>
      {onRetry ? (
        <Button variant="secondary" onClick={onRetry}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}

export { EmptyState, type EmptyStateProps } from "@gantry/design-system";
