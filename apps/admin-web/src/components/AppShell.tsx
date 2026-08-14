import { NavLink, Outlet } from 'react-router-dom';
import { Bot, LogOut, PanelLeft, Settings2 } from 'lucide-react';
import { IconButton } from '@gantry/design-system';
import { useAuth } from '../auth/AuthProvider';

export function AppShell() {
  const { user, signOut } = useAuth();
  const name = user?.profile.preferred_username ?? user?.profile.name ?? 'Administrator';
  return (
    <div className="admin-shell">
      <aside className="admin-sidebar">
        <div className="admin-brand"><span className="admin-brand-mark"><Bot size={18} /></span><span>Gantry Admin</span></div>
        <nav className="admin-nav" aria-label="Admin navigation">
          <span className="admin-nav-label">Agent management</span>
          <NavLink end to="/" className={({ isActive }) => `admin-nav-link ${isActive ? 'admin-nav-link-active' : ''}`}><PanelLeft size={17} /><span>Agents</span></NavLink>
          <span className="admin-nav-label admin-nav-label-spaced">Later</span>
          <span className="admin-nav-link admin-nav-link-disabled" aria-disabled="true" title="Not available in this release"><Settings2 size={17} /><span>Operations</span></span>
        </nav>
        <div className="admin-profile"><span className="admin-avatar">{name.slice(0, 1).toUpperCase()}</span><span>{name}</span><IconButton label="Sign out" onClick={() => void signOut()}><LogOut size={16} /></IconButton></div>
      </aside>
      <main className="admin-main"><Outlet /></main>
    </div>
  );
}
