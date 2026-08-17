import {
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  useEffect,
  useRef,
  useState,
} from "react";

export * from "./theme";

export const SPACING_UNIT = 8;

export type StatusLabel =
  | "Draft"
  | "Published"
  | "Running"
  | "Completed"
  | "Failed"
  | "Queued"
  | "Canceled"
  | "Pending"
  | "Valid"
  | "Invalid";

export type ButtonVariant =
  "primary" | "secondary" | "quiet" | "danger" | "accent";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  fullWidth?: boolean;
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
  const sizeClass = size ? `ds-button-${size}` : "";
  const fullWidthClass = fullWidth ? "ds-button-full" : "";

  return (
    <button
      {...props}
      disabled={disabled || isLoading}
      className={`ds-button ds-button-${variant} ${sizeClass} ${fullWidthClass} ${className}`.trim()}
    >
      {isLoading ? (
        <span
          aria-hidden="true"
          className="ds-spin"
          style={{
            display: "inline-block",
            width: "14px",
            height: "14px",
            border: "2px solid currentColor",
            borderRightColor: "transparent",
            borderRadius: "50%",
            flexShrink: 0,
          }}
        />
      ) : null}
      {children}
    </button>
  );
}

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string;
  children: ReactNode;
  variant?: "default" | "quiet" | "active";
  size?: "sm" | "md" | "lg";
}

export function IconButton({
  label,
  children,
  variant = "default",
  size,
  className = "",
  ...props
}: IconButtonProps) {
  const variantClass = variant !== "default" ? `ds-icon-button-${variant}` : "";
  const sizeClass = size ? `ds-icon-button-${size}` : "";

  return (
    <button
      {...props}
      aria-label={label}
      title={props.title ?? label}
      className={`ds-icon-button ${variantClass} ${sizeClass} ${className}`.trim()}
    >
      {children}
    </button>
  );
}

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
      <div
        style={{ position: "relative", display: "flex", alignItems: "center" }}
      >
        {icon ? (
          <span
            aria-hidden="true"
            style={{
              position: "absolute",
              left: "12px",
              display: "grid",
              placeItems: "center",
              color: "var(--ds-text-muted)",
              pointerEvents: "none",
            }}
          >
            {icon}
          </span>
        ) : null}
        <input
          {...props}
          id={inputId}
          className={`ds-input ${className}`.trim()}
          style={icon ? { paddingLeft: "38px", ...props.style } : props.style}
        />
      </div>
      {error ? (
        <span
          role="alert"
          style={{
            color: "var(--ds-danger)",
            fontSize: "12px",
            marginTop: "2px",
          }}
        >
          {error}
        </span>
      ) : null}
    </label>
  );
}

export function StatusMark({
  status,
  className = "",
}: {
  status: string;
  className?: string;
}) {
  const normalized = status.toLowerCase().replace(/_/g, "-");
  return (
    <span className={`ds-status ds-status-${normalized} ${className}`.trim()}>
      <span aria-hidden="true" className="ds-status-dot" />
      {status}
    </span>
  );
}

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
      className={`ds-card ${interactive ? "ds-card-interactive" : ""} ${className}`.trim()}
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
    <div {...props} className={`ds-card-header ${className}`.trim()}>
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
    <h3 {...props} className={`ds-card-title ${className}`.trim()}>
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
    <p {...props} className={`ds-card-description ${className}`.trim()}>
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
    <div {...props} className={`ds-card-content ${className}`.trim()}>
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
    <div {...props} className={`ds-card-footer ${className}`.trim()}>
      {children}
    </div>
  );
}

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
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const selectId =
    id ??
    (label
      ? `ds-select-${label.toLowerCase().replace(/\s+/g, "-")}`
      : undefined);

  const selectedOption = options.find((opt) => opt.value === value);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && isOpen) {
        setIsOpen(false);
        triggerRef.current?.focus();
      }
    }

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      document.addEventListener("keydown", handleKeyDown);
      return () => {
        document.removeEventListener("mousedown", handleClickOutside);
        document.removeEventListener("keydown", handleKeyDown);
      };
    }
  }, [isOpen]);

  const handleSelect = (val: string) => {
    onChange(val);
    setIsOpen(false);
    triggerRef.current?.focus();
  };

  return (
    <div
      className={`ds-field ds-select-wrapper ${className}`.trim()}
      ref={containerRef}
    >
      {label ? (
        <span
          className="ds-field-label"
          id={selectId ? `${selectId}-label` : undefined}
        >
          {label}
        </span>
      ) : null}

      {/* Synchronized hidden native select for standard form and test query compatibility */}
      <select
        id={selectId}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="ds-select-native"
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

      {/* Styled Custom Select Trigger */}
      <button
        ref={triggerRef}
        type="button"
        role="combobox"
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-labelledby={label && selectId ? `${selectId}-label` : undefined}
        disabled={disabled}
        onClick={() => setIsOpen((prev) => !prev)}
        className={`ds-input ds-select-trigger ${isOpen ? "ds-select-trigger-open" : ""}`}
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
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className={`ds-select-chevron ${isOpen ? "ds-select-chevron-open" : ""}`}
          aria-hidden="true"
        >
          <polyline points="6 9 12 15 18 9" />
        </svg>
      </button>

      {/* Floating Animated Dropdown Menu */}
      {isOpen ? (
        <ul role="listbox" className="ds-dropdown-menu ds-select-dropdown">
          {options.map((opt) => {
            const isSelected = opt.value === value;
            return (
              <li
                key={opt.value}
                role="option"
                aria-selected={isSelected}
                tabIndex={0}
                onClick={() => handleSelect(opt.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    handleSelect(opt.value);
                  }
                }}
                className={`ds-dropdown-item ${isSelected ? "ds-dropdown-item-selected" : ""}`}
              >
                <div className="ds-dropdown-item-content">
                  {opt.icon ? (
                    <span className="ds-dropdown-item-icon">{opt.icon}</span>
                  ) : null}
                  <span>{opt.label}</span>
                </div>
                {isSelected ? (
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="ds-dropdown-check"
                    aria-hidden="true"
                  >
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                ) : null}
              </li>
            );
          })}
        </ul>
      ) : null}
    </div>
  );
}

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
  align?: "left" | "right";
  className?: string;
}

export function DropdownMenu({
  trigger,
  items,
  align = "right",
  className = "",
}: DropdownMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && isOpen) {
        setIsOpen(false);
      }
    }

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      document.addEventListener("keydown", handleKeyDown);
      return () => {
        document.removeEventListener("mousedown", handleClickOutside);
        document.removeEventListener("keydown", handleKeyDown);
      };
    }
  }, [isOpen]);

  return (
    <div
      className={`ds-dropdown-wrapper ${className}`.trim()}
      ref={containerRef}
    >
      <div
        onClick={() => setIsOpen((prev) => !prev)}
        className="ds-dropdown-trigger-box"
      >
        {trigger}
      </div>

      {isOpen ? (
        <div
          role="menu"
          className={`ds-dropdown-menu ds-dropdown-menu-${align}`}
        >
          {items.map((item, idx) => {
            if (item === "divider") {
              return (
                <div
                  key={`divider-${idx}`}
                  className="ds-dropdown-divider"
                  role="separator"
                />
              );
            }

            return (
              <button
                key={item.id}
                type="button"
                role="menuitem"
                disabled={item.disabled}
                onClick={() => {
                  item.onClick?.();
                  setIsOpen(false);
                }}
                className={`ds-dropdown-item ${item.danger ? "ds-dropdown-item-danger" : ""}`}
              >
                {item.icon ? (
                  <span className="ds-dropdown-item-icon">{item.icon}</span>
                ) : null}
                <span>{item.label}</span>
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}

export type BadgeVariant =
  "default" | "success" | "warning" | "danger" | "neutral" | "info";
export type BadgeSize = "sm" | "md";

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
  size?: BadgeSize;
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
      className={`ds-badge ds-badge-${variant} ds-badge-${size} ${className}`.trim()}
    >
      {icon ? (
        <span
          aria-hidden="true"
          style={{ display: "inline-flex", alignItems: "center" }}
        >
          {icon}
        </span>
      ) : null}
      {children}
    </span>
  );
}

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
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && isOpen) {
        onClose();
      }
    }
    if (isOpen) {
      document.addEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "hidden";
      return () => {
        document.removeEventListener("keydown", handleKeyDown);
        document.body.style.overflow = "";
      };
    }
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return (
    <div
      className="ds-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="ds-modal-title"
      aria-describedby={description ? "ds-modal-desc" : undefined}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        className="ds-modal-container"
        style={maxWidth ? { maxWidth: `${maxWidth}px` } : undefined}
      >
        <header className="ds-modal-header">
          <div className="ds-modal-header-copy">
            <h2 id="ds-modal-title" className="ds-modal-title">
              {title}
            </h2>
            {description ? (
              <p id="ds-modal-desc" className="ds-modal-description">
                {description}
              </p>
            ) : null}
          </div>
          <button
            type="button"
            aria-label="Close dialog"
            onClick={onClose}
            className="ds-icon-button ds-icon-button-sm"
          >
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </header>
        <div className="ds-modal-body">{children}</div>
        {footer ? <footer className="ds-modal-footer">{footer}</footer> : null}
      </div>
    </div>
  );
}

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
    <div className={`ds-empty-state ${className}`.trim()}>
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

  const copyToClipboard = () => {
    void navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className={`ds-code-block ${className}`.trim()}>
      <div className="ds-code-header">
        <span>{language}</span>
        <button
          type="button"
          className="ds-button ds-button-quiet ds-button-sm"
          onClick={copyToClipboard}
          style={{ padding: "2px 8px", fontSize: "11px" }}
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
    <div className={`ds-tabs ${className}`.trim()} role="tablist">
      {tabs.map((tab) => {
        const isActive = tab.id === activeId;
        return (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => onChange(tab.id)}
            className={`ds-tab-btn ${isActive ? "ds-tab-active" : ""}`}
          >
            {tab.icon ? <span aria-hidden="true">{tab.icon}</span> : null}
            <span>{tab.label}</span>
            {tab.badge !== undefined ? (
              <span className="ds-tab-badge">{tab.badge}</span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}

/* --------------------------------------------------------------------------
   Shared Formatting Utilities
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
