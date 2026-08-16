import { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { Activity, Bot, Cable, ClipboardCheck, Database, FileSearch, Gauge, Layers3, LogOut, PanelLeft, PanelLeftClose, PanelLeftOpen, Plus, Shield, PlugZap, ServerCog } from 'lucide-react';
import { IconButton, ThemeToggle } from '@gantry/design-system';
import { useAuth } from '../auth/AuthProvider';

const ADMIN_SIDEBAR_STORAGE_KEY = 'gantry_admin_sidebar_collapsed';

export function AppShell() {
  const { user, signOut } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const name = user?.profile.preferred_username ?? user?.profile.name ?? 'Administrator';

  const [isCollapsed, setIsCollapsed] = useState(() => {
    try {
      return localStorage.getItem(ADMIN_SIDEBAR_STORAGE_KEY) === 'true';
    } catch {
      return false;
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(ADMIN_SIDEBAR_STORAGE_KEY, String(isCollapsed));
    } catch {
      // Ignore storage errors in restricted contexts
    }
  }, [isCollapsed]);

  const isNewAgentActive = location.pathname === '/new' || location.pathname === '/agents/new';

  return (
    <div className={`admin-shell ${isCollapsed ? 'admin-shell-collapsed' : ''}`}>
      <aside className={`admin-sidebar ${isCollapsed ? 'admin-sidebar-collapsed' : ''}`}>
        <div className="admin-sidebar-header">
          <div className="admin-brand">
            <span className="admin-brand-mark">
              <Bot size={18} strokeWidth={2.2} />
            </span>
            {!isCollapsed ? <span className="admin-brand-title">Gantry Admin</span> : null}
          </div>
          <IconButton
            label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            onClick={() => setIsCollapsed((prev) => !prev)}
            className="admin-sidebar-toggle-btn"
          >
            {isCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
          </IconButton>
        </div>

        {/* ChatGPT "+ New agent" Action Button */}
        <div className="admin-sidebar-action-wrap">
          {!isCollapsed ? (
            <button
              type="button"
              onClick={() => navigate('/agents/new')}
              className={`admin-new-agent-btn ${
                isNewAgentActive ? 'admin-new-agent-btn-active' : ''
              }`}
            >
              <div className="admin-new-agent-icon">
                <Plus size={15} strokeWidth={2.5} />
              </div>
              <span>New agent</span>
            </button>
          ) : (
            <IconButton
              label="New agent"
              onClick={() => navigate('/agents/new')}
              className={`admin-new-agent-btn-collapsed ${
                isNewAgentActive ? 'admin-new-agent-collapsed-active' : ''
              }`}
            >
              <Plus size={18} strokeWidth={2.5} />
            </IconButton>
          )}
        </div>

        <nav className="admin-nav" aria-label="Admin navigation">
          {!isCollapsed ? (
            <span className="admin-nav-label">Workspace</span>
          ) : (
            <div className="admin-nav-divider" />
          )}

          <NavLink end to="/" title={isCollapsed ? 'Overview' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><Gauge size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Overview</span> : null}</NavLink>
          <NavLink to="/agents" title={isCollapsed ? 'Agents' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><PanelLeft size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Agents</span> : null}</NavLink>

          {!isCollapsed ? <span className="admin-nav-label admin-nav-label-spaced">Configuration</span> : <div className="admin-nav-divider admin-nav-divider-spaced" />}
          <NavLink to="/skills" title={isCollapsed ? 'Skills' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><Layers3 size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Skills</span> : null}</NavLink>
          <NavLink to="/plugins" title={isCollapsed ? 'Plugins' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><Database size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Plugins</span> : null}</NavLink>
          <NavLink to="/tools" title={isCollapsed ? 'Tools' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><Cable size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Tools</span> : null}</NavLink>

          {!isCollapsed ? <span className="admin-nav-label admin-nav-label-spaced">Operate</span> : <div className="admin-nav-divider admin-nav-divider-spaced" />}
          <NavLink to="/runs" title={isCollapsed ? 'Runs' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><Activity size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Runs</span> : null}</NavLink>
          <NavLink to="/evaluations" title={isCollapsed ? 'Evaluations' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><ClipboardCheck size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Evaluations</span> : null}</NavLink>

          {!isCollapsed ? <span className="admin-nav-label admin-nav-label-spaced">Govern</span> : <div className="admin-nav-divider admin-nav-divider-spaced" />}
          <NavLink to="/audit" title={isCollapsed ? 'Audit' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><FileSearch size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Audit</span> : null}</NavLink>
          <NavLink to="/policies" title={isCollapsed ? 'Policies' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><Shield size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Policies</span> : null}</NavLink>
          <NavLink to="/integrations" title={isCollapsed ? 'Integrations' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><PlugZap size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Integrations</span> : null}</NavLink>

          {!isCollapsed ? <span className="admin-nav-label admin-nav-label-spaced">Platform</span> : <div className="admin-nav-divider admin-nav-divider-spaced" />}
          <NavLink to="/platform" title={isCollapsed ? 'Platform' : undefined} className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''} ${isCollapsed ? 'admin-nav-link-collapsed' : ''}`}><ServerCog size={17} className="admin-nav-icon" />{!isCollapsed ? <span>Platform</span> : null}</NavLink>
        </nav>

        {/* Sidebar Footer with Theme Toggle and Admin Profile */}
        <div className="admin-sidebar-footer">
          {!isCollapsed ? (
            <div className="admin-theme-section">
              <ThemeToggle variant="segmented" size="sm" />
            </div>
          ) : (
            <div className="admin-theme-section-collapsed">
              <ThemeToggle variant="icon" size="sm" />
            </div>
          )}

          <div className="admin-profile" title={name}>
            <div className="admin-profile-info">
              <span className="admin-avatar">{name.slice(0, 1).toUpperCase()}</span>
              {!isCollapsed ? <span className="admin-profile-name">{name}</span> : null}
            </div>
            <IconButton
              label="Sign out"
              onClick={() => void signOut()}
              className="admin-signout-btn"
            >
              <LogOut size={16} />
            </IconButton>
          </div>
        </div>
      </aside>

      <main className="admin-main">
        <Outlet />
      </main>
    </div>
  );
}
