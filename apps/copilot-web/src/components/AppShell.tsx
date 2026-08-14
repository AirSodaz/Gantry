import { NavLink, Outlet } from 'react-router-dom';
import { Bot, FileCheck2, ListTodo, LogOut, MessageSquarePlus, PackageOpen } from 'lucide-react';
import { IconButton } from '@gantry/design-system';
import { useAuth } from '../auth/AuthProvider';

const primaryLinks = [
  { to: '/', label: 'New task', icon: MessageSquarePlus, end: true },
  { to: '/agents', label: 'Agents', icon: Bot },
  { to: '/tasks', label: 'My tasks', icon: ListTodo },
];

export function AppShell() {
  const { user, signOut } = useAuth();
  const displayName = user?.profile.preferred_username ?? user?.profile.name ?? 'Copilot user';

  return (
    <div className="app-shell">
      <aside className="app-sidebar">
        <div className="brand-lockup">
          <div className="brand-mark" aria-hidden="true"><Bot size={18} strokeWidth={2.2} /></div>
          <span>Gantry Copilot</span>
        </div>
        <nav className="sidebar-nav" aria-label="Primary navigation">
          <span className="nav-section-label">Workspace</span>
          {primaryLinks.map(({ to, label, icon: Icon, end }) => (
            <NavLink key={to} to={to} end={end} className={({ isActive }) => `nav-link ${isActive ? 'nav-link-active' : ''}`}>
              <Icon size={17} aria-hidden="true" />
              <span>{label}</span>
            </NavLink>
          ))}
          <span className="nav-section-label nav-section-spaced">Governance</span>
          <NavLink to="/approvals" className={({ isActive }) => `nav-link ${isActive ? 'nav-link-active' : ''}`}>
            <FileCheck2 size={17} aria-hidden="true" />
            <span>Approvals</span>
          </NavLink>
          <span className="nav-link nav-link-disabled" title="Not available in this release" aria-disabled="true">
            <PackageOpen size={17} aria-hidden="true" />
            <span>Artifacts</span>
          </span>
        </nav>
        <div className="sidebar-footer">
          <div className="profile-row">
            <span className="profile-avatar" aria-hidden="true">{displayName.slice(0, 1).toUpperCase()}</span>
            <span className="profile-name">{displayName}</span>
          </div>
          <IconButton label="Sign out" onClick={() => void signOut()}>
            <LogOut size={16} />
          </IconButton>
        </div>
      </aside>
      <main className="app-main">
        <header className="mobile-header">
          <div className="brand-lockup">
            <div className="brand-mark" aria-hidden="true"><Bot size={17} /></div>
            <span>Gantry Copilot</span>
          </div>
          <IconButton label="Sign out" onClick={() => void signOut()}><LogOut size={16} /></IconButton>
        </header>
        <Outlet />
      </main>
    </div>
  );
}
