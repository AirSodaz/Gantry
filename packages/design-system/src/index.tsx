import {
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  useState,
} from "react";
import * as Dialog from "@radix-ui/react-dialog";
import * as RadixSelect from "@radix-ui/react-select";
import * as RadixDropdown from "@radix-ui/react-dropdown-menu";
import * as RadixTabs from "@radix-ui/react-tabs";
import { cva, type VariantProps } from "class-variance-authority";
import { Check, ChevronDown, X } from "lucide-react";
import { cn } from "./utils";

export * from "./theme";
export * from "./utils";

export const SPACING_UNIT = 8;

/* --------------------------------------------------------------------------
   1. State Vocabulary & StatusMark
   -------------------------------------------------------------------------- */

export type RunState =
  | "Draft"
  | "In review"
  | "Published"
  | "Deprecated"
  | "Queued"
  | "Provisioning"
  | "Running"
  | "Awaiting approval"
  | "Awaiting requester input"
  | "Suspended"
  | "Canceling"
  | "Completed"
  | "Failed"
  | "Canceled"
  | "Expired"
  | "Unknown outcome";

export type GovernanceState =
  | "Requested"
  | "Processing"
  | "Evaluating"
  | "Ready"
  | "Pending"
  | "Blocked"
  | "Quarantined"
  | "Active"
  | "Released"
  | "Retired"
  | "Available"
  | "Valid"
  | "Invalid"
  | "Passed"
  | "Approved"
  | "Rejected"
  | "Disabled"
  | "Draining"
  | "Proposed";

export type StatusLabel = RunState | GovernanceState | (string & {});

export function formatStatusLabel(status?: string | null): string {
  if (!status) return "Unknown";
  return status
    .replace(/[_-]+/g, " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

export function StatusMark({
  status,
  className = "",
}: {
  status?: string | null;
  className?: string;
}) {
  const safeStatus = status || "Unknown";
  const normalized = safeStatus.toLowerCase().replace(/[\s_]+/g, "-");
  return (
    <span className={cn("ds-status", `ds-status-${normalized}`, className)}>
      <span aria-hidden="true" className="ds-status-dot" />
      {status}
    </span>
  );
}

/* --------------------------------------------------------------------------
   2. Button & IconButton (CVA)
   -------------------------------------------------------------------------- */

export const buttonVariants = cva(
  "ds-button inline-flex items-center justify-center font-medium transition-all duration-150 cursor-pointer disabled:cursor-not-allowed focus-visible:outline-2 focus-visible:outline-offset-2",
  {
    variants: {
      variant: {
        primary: "ds-button-primary",
        secondary: "ds-button-secondary",
        quiet: "ds-button-quiet",
        danger: "ds-button-danger",
        accent: "ds-button-accent",
      },
      size: {
        sm: "ds-button-sm",
        md: "",
        lg: "ds-button-lg",
      },
      fullWidth: {
        true: "ds-button-full w-full",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  },
);

export type ButtonVariant =
  "primary" | "secondary" | "quiet" | "danger" | "accent";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps
  extends
    ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  isLoading?: boolean;
}

export function Button({
  children,
  variant = "primary",
  size,
  fullWidth = false,
  isLoading = false,
  disabled,
  className = "",
  ...props
}: ButtonProps) {
  return (
    <button
      {...props}
      disabled={disabled || isLoading}
      className={cn(
        buttonVariants({
          variant: variant as ButtonVariant,
          size: size as ButtonSize,
          fullWidth: Boolean(fullWidth),
        }),
        className,
      )}
    >
      {isLoading ? (
        <span
          aria-hidden="true"
          className="ds-spin inline-block w-3.5 h-3.5 border-2 border-current border-r-transparent rounded-full shrink-0"
        />
      ) : null}
      {children}
    </button>
  );
}

export const iconButtonVariants = cva("ds-icon-button", {
  variants: {
    variant: {
      default: "",
      quiet: "ds-icon-button-quiet",
      active: "ds-icon-button-active",
    },
    size: {
      sm: "ds-icon-button-sm",
      md: "",
      lg: "ds-icon-button-lg",
    },
  },
  defaultVariants: {
    variant: "default",
    size: "md",
  },
});

export interface IconButtonProps
  extends
    ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof iconButtonVariants> {
  label: string;
  children: ReactNode;
}

export function IconButton({
  label,
  children,
  variant = "default",
  size,
  className = "",
  ...props
}: IconButtonProps) {
  return (
    <button
      {...props}
      aria-label={label}
      title={props.title ?? label}
      className={cn(iconButtonVariants({ variant, size }), className)}
    >
      {children}
    </button>
  );
}

/* --------------------------------------------------------------------------
   3. Form Inputs & Text Fields
   -------------------------------------------------------------------------- */

export interface TextInputProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
  icon?: ReactNode;
}

export function TextInput({
  label,
  error,
  icon,
  className = "",
  id,
  ...props
}: TextInputProps) {
  const inputId = id ?? `ds-input-${label.toLowerCase().replace(/\s+/g, "-")}`;

  return (
    <label className="ds-field" htmlFor={inputId}>
      <span className="ds-field-label">{label}</span>
      <div className="relative flex items-center">
        {icon ? (
          <span
            aria-hidden="true"
            className="absolute left-3 grid place-items-center text-[var(--ds-text-muted)] pointer-events-none"
          >
            {icon}
          </span>
        ) : null}
        <input
          {...props}
          id={inputId}
          className={cn("ds-input", className)}
          style={icon ? { paddingLeft: "38px", ...props.style } : props.style}
        />
      </div>
      {error ? (
        <span
          role="alert"
          className="text-xs text-[var(--ds-danger)] mt-0.5 block"
        >
          {error}
        </span>
      ) : null}
    </label>
  );
}

/* --------------------------------------------------------------------------
   4. Card Components (CVA)
   -------------------------------------------------------------------------- */

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
}

export function Card({
  children,
  interactive = false,
  className = "",
  ...props
}: CardProps) {
  return (
    <div
      {...props}
      className={cn("ds-card", interactive && "ds-card-interactive", className)}
    >
      {children}
    </div>
  );
}

export function CardHeader({
  children,
  className = "",
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div {...props} className={cn("ds-card-header", className)}>
      {children}
    </div>
  );
}

export function CardTitle({
  children,
  className = "",
  ...props
}: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3 {...props} className={cn("ds-card-title", className)}>
      {children}
    </h3>
  );
}

export function CardDescription({
  children,
  className = "",
  ...props
}: HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p {...props} className={cn("ds-card-description", className)}>
      {children}
    </p>
  );
}

export function CardContent({
  children,
  className = "",
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div {...props} className={cn("ds-card-content", className)}>
      {children}
    </div>
  );
}

export function CardFooter({
  children,
  className = "",
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div {...props} className={cn("ds-card-footer", className)}>
      {children}
    </div>
  );
}

/* --------------------------------------------------------------------------
   5. Select Component (Radix Select Primitive + Native Sync)
   -------------------------------------------------------------------------- */

export interface SelectOption {
  value: string;
  label: string;
  icon?: ReactNode;
}

export interface SelectProps extends Omit<
  SelectHTMLAttributes<HTMLSelectElement>,
  "onChange" | "value"
> {
  label?: string;
  options: SelectOption[];
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}

export function Select({
  label,
  options,
  value,
  onChange,
  placeholder = "Select an option",
  disabled = false,
  className = "",
  id,
}: SelectProps) {
  const selectId =
    id ??
    (label
      ? `ds-select-${label.toLowerCase().replace(/\s+/g, "-")}`
      : undefined);

  const selectedOption = options.find((opt) => opt.value === value);

  return (
    <div className={cn("ds-field ds-select-wrapper", className)}>
      {label ? (
        <span
          className="ds-field-label"
          id={selectId ? `${selectId}-label` : undefined}
        >
          {label}
        </span>
      ) : null}

      {/* Synchronized hidden native select for standard HTML form/test queries compatibility */}
      <select
        id={selectId}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="ds-select-native"
        tabIndex={-1}
      >
        <option value="" disabled>
          {placeholder}
        </option>
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>

      {/* Radix UI Headless Select */}
      <RadixSelect.Root
        value={value}
        onValueChange={onChange}
        disabled={disabled}
      >
        <RadixSelect.Trigger
          aria-labelledby={label && selectId ? `${selectId}-label` : undefined}
          className="ds-input ds-select-trigger"
        >
          <div className="ds-select-trigger-content">
            {selectedOption?.icon ? (
              <span className="ds-select-icon">{selectedOption.icon}</span>
            ) : null}
            <span
              className={
                selectedOption ? "ds-select-value" : "ds-select-placeholder"
              }
            >
              {selectedOption ? selectedOption.label : placeholder}
            </span>
          </div>
          <RadixSelect.Icon asChild>
            <ChevronDown
              size={16}
              className="ds-select-chevron"
              aria-hidden="true"
            />
          </RadixSelect.Icon>
        </RadixSelect.Trigger>

        <RadixSelect.Portal>
          <RadixSelect.Content
            position="popper"
            sideOffset={4}
            className="ds-dropdown-menu ds-select-dropdown z-50"
          >
            <RadixSelect.Viewport>
              {options.map((opt) => (
                <RadixSelect.Item
                  key={opt.value}
                  value={opt.value}
                  className="ds-dropdown-item"
                >
                  <div className="ds-dropdown-item-content">
                    {opt.icon ? (
                      <span className="ds-dropdown-item-icon">{opt.icon}</span>
                    ) : null}
                    <RadixSelect.ItemText>{opt.label}</RadixSelect.ItemText>
                  </div>
                  <RadixSelect.ItemIndicator asChild>
                    <Check
                      size={14}
                      className="ds-dropdown-check"
                      aria-hidden="true"
                    />
                  </RadixSelect.ItemIndicator>
                </RadixSelect.Item>
              ))}
            </RadixSelect.Viewport>
          </RadixSelect.Content>
        </RadixSelect.Portal>
      </RadixSelect.Root>
    </div>
  );
}

/* --------------------------------------------------------------------------
   6. DropdownMenu Component (Radix DropdownMenu Primitive)
   -------------------------------------------------------------------------- */

export interface DropdownMenuItem {
  id: string;
  label: string;
  icon?: ReactNode;
  danger?: boolean;
  disabled?: boolean;
  onClick?: () => void;
}

export interface DropdownMenuProps {
  trigger: ReactNode;
  items: (DropdownMenuItem | "divider")[];
  align?: "start" | "end" | "center" | "left" | "right";
  className?: string;
}

export function DropdownMenu({
  trigger,
  items,
  align = "right",
  className = "",
}: DropdownMenuProps) {
  const resolvedAlign =
    align === "left" ? "start" : align === "right" ? "end" : align;

  return (
    <RadixDropdown.Root>
      <RadixDropdown.Trigger asChild className={className}>
        <div className="ds-dropdown-trigger-box">{trigger}</div>
      </RadixDropdown.Trigger>

      <RadixDropdown.Portal>
        <RadixDropdown.Content
          align={resolvedAlign as "start" | "end" | "center"}
          sideOffset={4}
          className="ds-dropdown-menu z-50"
        >
          {items.map((item, idx) => {
            if (item === "divider") {
              return (
                <RadixDropdown.Separator
                  key={`divider-${idx}`}
                  className="ds-dropdown-divider"
                />
              );
            }

            return (
              <RadixDropdown.Item
                key={item.id}
                disabled={item.disabled}
                onSelect={() => item.onClick?.()}
                className={cn(
                  "ds-dropdown-item",
                  item.danger && "ds-dropdown-item-danger",
                )}
              >
                {item.icon ? (
                  <span className="ds-dropdown-item-icon">{item.icon}</span>
                ) : null}
                <span>{item.label}</span>
              </RadixDropdown.Item>
            );
          })}
        </RadixDropdown.Content>
      </RadixDropdown.Portal>
    </RadixDropdown.Root>
  );
}

/* --------------------------------------------------------------------------
   7. Badge Component (CVA)
   -------------------------------------------------------------------------- */

export const badgeVariants = cva(
  "ds-badge inline-flex items-center font-medium whitespace-nowrap",
  {
    variants: {
      variant: {
        default: "ds-badge-default",
        success: "ds-badge-success",
        warning: "ds-badge-warning",
        danger: "ds-badge-danger",
        neutral: "ds-badge-neutral",
        info: "ds-badge-info",
      },
      size: {
        sm: "ds-badge-sm",
        md: "",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "md",
    },
  },
);

export type BadgeVariant =
  "default" | "success" | "warning" | "danger" | "neutral" | "info";
export type BadgeSize = "sm" | "md";

export interface BadgeProps
  extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {
  icon?: ReactNode;
}

export function Badge({
  children,
  variant = "default",
  size = "md",
  icon,
  className = "",
  ...props
}: BadgeProps) {
  return (
    <span
      {...props}
      className={cn(
        badgeVariants({
          variant: variant as BadgeVariant,
          size: size as BadgeSize,
        }),
        className,
      )}
    >
      {icon ? (
        <span aria-hidden="true" className="inline-flex items-center">
          {icon}
        </span>
      ) : null}
      {children}
    </span>
  );
}

/* --------------------------------------------------------------------------
   8. Modal Component (Radix Dialog Primitive)
   -------------------------------------------------------------------------- */

export interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
  maxWidth?: number;
}

export function Modal({
  isOpen,
  onClose,
  title,
  description,
  children,
  footer,
  maxWidth,
}: ModalProps) {
  return (
    <Dialog.Root
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="ds-modal-overlay z-50" />
        <Dialog.Content
          className="ds-modal-container z-50"
          style={maxWidth ? { maxWidth: `${maxWidth}px` } : undefined}
          aria-describedby={description ? "ds-modal-desc" : undefined}
        >
          <header className="ds-modal-header">
            <div className="ds-modal-header-copy">
              <Dialog.Title id="ds-modal-title" className="ds-modal-title">
                {title}
              </Dialog.Title>
              {description ? (
                <Dialog.Description
                  id="ds-modal-desc"
                  className="ds-modal-description"
                >
                  {description}
                </Dialog.Description>
              ) : null}
            </div>
            <Dialog.Close asChild>
              <button
                type="button"
                aria-label="Close dialog"
                className="ds-icon-button ds-icon-button-sm"
              >
                <X size={16} aria-hidden="true" />
              </button>
            </Dialog.Close>
          </header>
          <div className="ds-modal-body">{children}</div>
          {footer ? (
            <footer className="ds-modal-footer">{footer}</footer>
          ) : null}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/* --------------------------------------------------------------------------
   9. EmptyState Component
   -------------------------------------------------------------------------- */

export interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className = "",
}: EmptyStateProps) {
  return (
    <div className={cn("ds-empty-state", className)}>
      {icon ? (
        <div className="ds-empty-icon" aria-hidden="true">
          {icon}
        </div>
      ) : null}
      <h3 className="ds-empty-title">{title}</h3>
      {description ? <p className="ds-empty-desc">{description}</p> : null}
      {action ? <div className="ds-empty-action">{action}</div> : null}
    </div>
  );
}

/* --------------------------------------------------------------------------
   10. CodeBlock Component
   -------------------------------------------------------------------------- */

export interface CodeBlockProps {
  code: string;
  language?: string;
  className?: string;
  maxHeight?: number | string;
}

export function CodeBlock({
  code,
  language = "json",
  className = "",
  maxHeight,
}: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  const copyToClipboard = async () => {
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      }
    } catch {
      // Clipboard access may fail in restricted/unsupported contexts
    }
  };

  return (
    <div className={cn("ds-code-block", className)}>
      <div className="ds-code-header">
        <span>{language}</span>
        <button
          type="button"
          className="ds-button ds-button-quiet ds-button-sm text-[11px] px-2 py-0.5"
          onClick={copyToClipboard}
        >
          {copied ? "Copied" : "Copy"}
        </button>
      </div>
      <pre
        className="ds-code-pre"
        style={maxHeight ? { maxHeight, overflowY: "auto" } : undefined}
      >
        <code>{code}</code>
      </pre>
    </div>
  );
}

/* --------------------------------------------------------------------------
   11. Tabs Component (Radix Tabs Primitive)
   -------------------------------------------------------------------------- */

export interface TabItem {
  id: string;
  label: string;
  icon?: ReactNode;
  badge?: number | string;
}

export interface TabsProps {
  tabs: TabItem[];
  activeId: string;
  onChange: (id: string) => void;
  className?: string;
}

export function Tabs({ tabs, activeId, onChange, className = "" }: TabsProps) {
  return (
    <RadixTabs.Root
      value={activeId}
      onValueChange={onChange}
      className={cn("ds-tabs", className)}
    >
      <RadixTabs.List className="flex items-center gap-1">
        {tabs.map((tab) => (
          <RadixTabs.Trigger key={tab.id} value={tab.id} className="ds-tab-btn">
            {tab.icon ? <span aria-hidden="true">{tab.icon}</span> : null}
            <span>{tab.label}</span>
            {tab.badge !== undefined ? (
              <span className="ds-tab-badge">{tab.badge}</span>
            ) : null}
          </RadixTabs.Trigger>
        ))}
      </RadixTabs.List>
    </RadixTabs.Root>
  );
}

/* --------------------------------------------------------------------------
   12. Shared Formatting Utilities
   -------------------------------------------------------------------------- */

export function formatDate(value?: string | number | Date | null): string {
  if (!value) return "Not available";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? String(value)
    : new Intl.DateTimeFormat(undefined, {
        dateStyle: "medium",
        timeStyle: "short",
      }).format(date);
}

export function formatTime(value?: string | number | Date | null): string {
  if (!value) return "Not recorded";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString();
}

export function formatAge(value?: string | number | Date | null): string {
  if (!value) return "recently";
  const time = new Date(value).getTime();
  if (Number.isNaN(time)) return String(value);
  const minutes = Math.max(0, Math.floor((Date.now() - time) / 60_000));
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export function shortHash(value?: string | null, length = 12): string {
  if (!value) return "";
  return value.replace(/^sha256:/, "").slice(0, length);
}

export function formatBytes(bytes?: number | null): string {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes))
    return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
