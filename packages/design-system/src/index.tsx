import {
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  useEffect,
  useRef,
  useState,
} from 'react';

export * from './theme';

export const SPACING_UNIT = 8;

export type StatusLabel =
  | 'Draft'
  | 'Published'
  | 'Running'
  | 'Completed'
  | 'Failed'
  | 'Queued'
  | 'Canceled'
  | 'Pending'
  | 'Valid'
  | 'Invalid';

export type ButtonVariant = 'primary' | 'secondary' | 'quiet' | 'danger' | 'accent';
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  fullWidth?: boolean;
  isLoading?: boolean;
}

export function Button({
  children,
  variant = 'primary',
  size,
  fullWidth = false,
  isLoading = false,
  disabled,
  className = '',
  ...props
}: ButtonProps) {
  const sizeClass = size ? `ds-button-${size}` : '';
  const fullWidthClass = fullWidth ? 'ds-button-full' : '';

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
            display: 'inline-block',
            width: '14px',
            height: '14px',
            border: '2px solid currentColor',
            borderRightColor: 'transparent',
            borderRadius: '50%',
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
  variant?: 'default' | 'quiet' | 'active';
  size?: 'sm' | 'md' | 'lg';
}

export function IconButton({
  label,
  children,
  variant = 'default',
  size,
  className = '',
  ...props
}: IconButtonProps) {
  const variantClass = variant !== 'default' ? `ds-icon-button-${variant}` : '';
  const sizeClass = size ? `ds-icon-button-${size}` : '';

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
  className = '',
  id,
  ...props
}: TextInputProps) {
  const inputId = id ?? `ds-input-${label.toLowerCase().replace(/\s+/g, '-')}`;

  return (
    <label className="ds-field" htmlFor={inputId}>
      <span className="ds-field-label">{label}</span>
      <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
        {icon ? (
          <span
            aria-hidden="true"
            style={{
              position: 'absolute',
              left: '12px',
              display: 'grid',
              placeItems: 'center',
              color: 'var(--ds-text-muted)',
              pointerEvents: 'none',
            }}
          >
            {icon}
          </span>
        ) : null}
        <input
          {...props}
          id={inputId}
          className={`ds-input ${className}`.trim()}
          style={icon ? { paddingLeft: '38px', ...props.style } : props.style}
        />
      </div>
      {error ? (
        <span
          role="alert"
          style={{
            color: 'var(--ds-danger)',
            fontSize: '12px',
            marginTop: '2px',
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
  className = '',
}: {
  status: string;
  className?: string;
}) {
  const normalized = status.toLowerCase().replace(/_/g, '-');
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

export function Card({ children, interactive = false, className = '', ...props }: CardProps) {
  return (
    <div
      {...props}
      className={`ds-card ${interactive ? 'ds-card-interactive' : ''} ${className}`.trim()}
    >
      {children}
    </div>
  );
}

export function CardHeader({ children, className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div {...props} className={`ds-card-header ${className}`.trim()}>
      {children}
    </div>
  );
}

export function CardTitle({ children, className = '', ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h3 {...props} className={`ds-card-title ${className}`.trim()}>
      {children}
    </h3>
  );
}

export function CardDescription({ children, className = '', ...props }: HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p {...props} className={`ds-card-description ${className}`.trim()}>
      {children}
    </p>
  );
}

export function CardContent({ children, className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div {...props} className={`ds-card-content ${className}`.trim()}>
      {children}
    </div>
  );
}

export function CardFooter({ children, className = '', ...props }: HTMLAttributes<HTMLDivElement>) {
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

export interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'onChange' | 'value'> {
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
  placeholder = 'Select an option',
  disabled = false,
  className = '',
  id,
}: SelectProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const selectId = id ?? (label ? `ds-select-${label.toLowerCase().replace(/\s+/g, '-')}` : undefined);

  const selectedOption = options.find((opt) => opt.value === value);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape' && isOpen) {
        setIsOpen(false);
        triggerRef.current?.focus();
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      document.addEventListener('keydown', handleKeyDown);
      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
        document.removeEventListener('keydown', handleKeyDown);
      };
    }
  }, [isOpen]);

  const handleSelect = (val: string) => {
    onChange(val);
    setIsOpen(false);
    triggerRef.current?.focus();
  };

  return (
    <div className={`ds-field ds-select-wrapper ${className}`.trim()} ref={containerRef}>
      {label ? (
        <span className="ds-field-label" id={selectId ? `${selectId}-label` : undefined}>
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
        className={`ds-input ds-select-trigger ${isOpen ? 'ds-select-trigger-open' : ''}`}
      >
        <div className="ds-select-trigger-content">
          {selectedOption?.icon ? (
            <span className="ds-select-icon">{selectedOption.icon}</span>
          ) : null}
          <span className={selectedOption ? 'ds-select-value' : 'ds-select-placeholder'}>
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
          className={`ds-select-chevron ${isOpen ? 'ds-select-chevron-open' : ''}`}
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
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    handleSelect(opt.value);
                  }
                }}
                className={`ds-dropdown-item ${isSelected ? 'ds-dropdown-item-selected' : ''}`}
              >
                <div className="ds-dropdown-item-content">
                  {opt.icon ? <span className="ds-dropdown-item-icon">{opt.icon}</span> : null}
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
  items: (DropdownMenuItem | 'divider')[];
  align?: 'left' | 'right';
  className?: string;
}

export function DropdownMenu({
  trigger,
  items,
  align = 'right',
  className = '',
}: DropdownMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape' && isOpen) {
        setIsOpen(false);
      }
    }

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      document.addEventListener('keydown', handleKeyDown);
      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
        document.removeEventListener('keydown', handleKeyDown);
      };
    }
  }, [isOpen]);

  return (
    <div className={`ds-dropdown-wrapper ${className}`.trim()} ref={containerRef}>
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
            if (item === 'divider') {
              return <div key={`divider-${idx}`} className="ds-dropdown-divider" role="separator" />;
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
                className={`ds-dropdown-item ${item.danger ? 'ds-dropdown-item-danger' : ''}`}
              >
                {item.icon ? <span className="ds-dropdown-item-icon">{item.icon}</span> : null}
                <span>{item.label}</span>
              </button>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
