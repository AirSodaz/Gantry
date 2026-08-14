import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from 'react';

export const SPACING_UNIT = 8;

export type StatusLabel = 'Draft' | 'Published' | 'Running' | 'Completed' | 'Failed';
export type ButtonVariant = 'primary' | 'secondary' | 'quiet' | 'danger';

export function Button({
  children,
  variant = 'primary',
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: ButtonVariant }) {
  return (
    <button {...props} className={`ds-button ds-button-${variant} ${props.className ?? ''}`}>
      {children}
    </button>
  );
}

export function IconButton({
  label,
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { label: string; children: ReactNode }) {
  return (
    <button
      {...props}
      aria-label={label}
      className={`ds-icon-button ${props.className ?? ''}`}
      title={props.title ?? label}
    >
      {children}
    </button>
  );
}

export function TextInput({
  label,
  ...props
}: InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return (
    <label className="ds-field">
      <span className="ds-field-label">{label}</span>
      <input {...props} className={`ds-input ${props.className ?? ''}`} />
    </label>
  );
}

export function StatusMark({ status }: { status: string }) {
  const normalized = status.toLowerCase().replace(/_/g, '-');
  return (
    <span className={`ds-status ds-status-${normalized}`}>
      <span aria-hidden="true" className="ds-status-dot" />
      {status}
    </span>
  );
}
