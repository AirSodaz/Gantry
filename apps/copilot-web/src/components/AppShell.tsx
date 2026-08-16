import { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  Bot,
  FileCheck2,
  ListTodo,
  LogOut,
  Menu,
  PackageOpen,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  X,
} from 'lucide-react';
import { IconButton, ThemeToggle } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useAuth } from '../auth/AuthProvider';
import { useCopilotApi } from '../api/ApiProvider';

const SIDEBAR_STORAGE_KEY = 'gantry_sidebar_collapsed';

export function AppShell() {
  const { user, signOut } = useAuth();
  const api = useCopilotApi();
  const navigate = useNavigate();
  const location = useLocation();
  const displayName = user?.profile.preferred_username ?? user?.profile.name ?? 'Copilot user';

  const [isCollapsed, setIsCollapsed] = useState(() => {
    try {
      return localStorage.getItem(SIDEBAR_STORAGE_KEY) === 'true';
    } catch {
      return false;
    }
  });

  const [isMobileOpen, setIsMobileOpen] = useState(false);

  useEffect(() => {
    try {
      localStorage.setItem(SIDEBAR_STORAGE_KEY, String(isCollapsed));
    } catch {
      // Ignore storage errors in restricted contexts
    }
  }, [isCollapsed]);

  const toggleCollapse = () => setIsCollapsed((prev) => !prev);
  const closeMobile = () => setIsMobileOpen(false);
  const isNewTaskActive = location.pathname === '/' || location.pathname === '';
  const approvals = useQuery({ queryKey: ['approvals', 'shell'], queryFn: () => api.listApprovals(), refetchInterval: 30_000 });
  const pendingApprovals = approvals.data?.items.length ?? 0;

  return (
    <div
      className={`app-shell ${isCollapsed ? 'app-shell-collapsed' : ''} ${
        isMobileOpen ? 'app-shell-mobile-open' : ''
      }`}
    >
      {/* Mobile Backdrop Overlay */}
      {isMobileOpen ? (
        <div
          className="mobile-backdrop"
          onClick={closeMobile}
          aria-hidden="true"
        />
      ) : null}

      <aside className={`app-sidebar ${isCollapsed ? 'app-sidebar-collapsed' : ''}`}>
        <div className="sidebar-header">
          <div className="brand-lockup">
            <div className="brand-mark" aria-hidden="true">
              <Bot size={18} strokeWidth={2.2} />
            </div>
            {!isCollapsed ? <span className="brand-title">Gantry Copilot</span> : null}
          </div>
          <IconButton
            label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            onClick={toggleCollapse}
            className="sidebar-toggle-btn"
          >
            {isCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
          </IconButton>
        </div>

        {/* ChatGPT "+ New task" Action Button */}
        <div className="sidebar-action-wrap">
          {!isCollapsed ? (
            <button
              type="button"
              onClick={() => {
                navigate('/');
                closeMobile();
              }}
              className={`chatgpt-new-task-btn ${isNewTaskActive ? 'chatgpt-new-task-btn-active' : ''}`}
            >
              <div className="chatgpt-new-task-icon">
                <Plus size={15} strokeWidth={2.5} />
              </div>
              <span>New task</span>
            </button>
          ) : (
            <IconButton
              label="New task"
              onClick={() => {
                navigate('/');
                closeMobile();
              }}
              className={`chatgpt-new-task-btn-collapsed ${
                isNewTaskActive ? 'chatgpt-new-task-collapsed-active' : ''
              }`}
            >
              <Plus size={18} strokeWidth={2.5} />
            </IconButton>
          )}
        </div>

        <nav className="sidebar-nav" aria-label="Primary navigation">
          {!isCollapsed ? (
            <span className="nav-section-label">Workspace</span>
          ) : (
            <div className="nav-section-divider" />
          )}

          <NavLink
            to="/agents"
            onClick={closeMobile}
            title={isCollapsed ? 'Agents' : undefined}
            className={({ isActive }) =>
              `nav-link ${isActive ? 'nav-link-active' : ''} ${
                isCollapsed ? 'nav-link-collapsed' : ''
              }`
            }
          >
            <Bot size={17} aria-hidden="true" className="nav-link-icon" />
            {!isCollapsed ? <span>Agents</span> : null}
          </NavLink>

          <NavLink
            to="/tasks"
            onClick={closeMobile}
            title={isCollapsed ? 'My tasks' : undefined}
            className={({ isActive }) =>
              `nav-link ${isActive ? 'nav-link-active' : ''} ${
                isCollapsed ? 'nav-link-collapsed' : ''
              }`
            }
          >
            <ListTodo size={17} aria-hidden="true" className="nav-link-icon" />
            {!isCollapsed ? <span>My tasks</span> : null}
          </NavLink>

          {!isCollapsed ? (
            <span className="nav-section-label nav-section-spaced">Governance</span>
          ) : (
            <div className="nav-section-divider nav-section-spaced" />
          )}

          <NavLink
            to="/approvals"
            onClick={closeMobile}
            title={isCollapsed ? 'Approvals' : undefined}
            className={({ isActive }) =>
              `nav-link ${isActive ? 'nav-link-active' : ''} ${
                isCollapsed ? 'nav-link-collapsed' : ''
              }`
            }
          >
            <FileCheck2 size={17} aria-hidden="true" className="nav-link-icon" />
            {!isCollapsed ? <span>Approvals</span> : null}
            {pendingApprovals > 0 ? <span className="nav-count" aria-label={`${pendingApprovals} pending approvals`}>{pendingApprovals > 99 ? '99+' : pendingApprovals}</span> : null}
          </NavLink>

          <NavLink
            to="/artifacts"
            onClick={closeMobile}
            title={isCollapsed ? 'Artifacts' : undefined}
            className={({ isActive }) =>
              `nav-link ${isActive ? 'nav-link-active' : ''} ${
                isCollapsed ? 'nav-link-collapsed' : ''
              }`
            }
          >
            <PackageOpen size={17} aria-hidden="true" className="nav-link-icon" />
            {!isCollapsed ? <span>Artifacts</span> : null}
          </NavLink>
        </nav>

        {/* Sidebar Footer with Theme Toggle and User Profile */}
        <div className="sidebar-footer">
          {!isCollapsed ? (
            <div className="sidebar-theme-section">
              <ThemeToggle variant="segmented" size="sm" />
            </div>
          ) : (
            <div className="sidebar-theme-section-collapsed">
              <ThemeToggle variant="icon" size="sm" />
            </div>
          )}

          <div className="profile-row-container">
            <div className="profile-row" title={displayName}>
              <span className="profile-avatar" aria-hidden="true">
                {displayName.slice(0, 1).toUpperCase()}
              </span>
              {!isCollapsed ? <span className="profile-name">{displayName}</span> : null}
            </div>
            <IconButton
              label="Sign out"
              onClick={() => void signOut()}
              className="signout-btn"
            >
              <LogOut size={16} />
            </IconButton>
          </div>
        </div>
      </aside>

      <main className="app-main">
        <header className="mobile-header">
          <IconButton
            label="Open menu"
            onClick={() => setIsMobileOpen((prev) => !prev)}
          >
            {isMobileOpen ? <X size={18} /> : <Menu size={18} />}
          </IconButton>
          <div className="brand-lockup">
            <div className="brand-mark" aria-hidden="true">
              <Bot size={17} />
            </div>
            <span className="brand-title">Gantry</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <ThemeToggle variant="icon" size="sm" />
            <IconButton label="Sign out" onClick={() => void signOut()}>
              <LogOut size={16} />
            </IconButton>
          </div>
        </header>

        <div className="app-main-content">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
