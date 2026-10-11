import React from 'react';
import {
  LayoutDashboard,
  Shirt,
  Users,
  Tag,
  Activity,
  BookOpen,
  Building2,
  CheckCircle2,
  AlertCircle,
  ShieldCheck,
  ArrowRightLeft,
} from 'lucide-react';
import { ViewMode, Tenant, HealthCheckResult, User } from '../types';
import { ThemeToggle } from './ThemeToggle';

interface SidebarProps {
  currentView: ViewMode;
  onSelectView: (view: ViewMode) => void;
  tenant?: Tenant;
  currentUser: User;
  onOpenLoginModal: () => void;
  health: HealthCheckResult | null;
  isOpenMobile: boolean;
  onCloseMobile: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  currentView,
  onSelectView,
  tenant,
  currentUser,
  onOpenLoginModal,
  health,
  isOpenMobile,
  onCloseMobile,
}) => {
  const navItems: { id: ViewMode; label: string; icon: React.ElementType; badge?: string; roles?: string[] }[] = [
    { id: 'dashboard', label: 'Tableau de bord', icon: LayoutDashboard },
    { id: 'orders', label: 'Commandes & Linge', icon: Shirt, badge: '5 actives' },
    { id: 'customers', label: 'Fichier Clients', icon: Users, roles: ['owner', 'manager', 'cashier', 'superadmin'] },
    { id: 'services', label: 'Prestations & Tarifs', icon: Tag, roles: ['owner', 'manager', 'superadmin'] },
    { id: 'superadmin', label: 'Plateforme SaaS', icon: ShieldCheck, roles: ['superadmin'], badge: '12 pressings' },
    { id: 'system-health', label: 'Santé & API Go', icon: Activity },
    { id: 'docs', label: 'Documentation (Mac)', icon: BookOpen },
  ];

  const visibleNavItems = navItems.filter((item) => {
    if (!item.roles) return true;
    return item.roles.includes(currentUser.role);
  });

  const handleNavClick = (view: ViewMode) => {
    onSelectView(view);
    onCloseMobile();
  };

  return (
    <>
      {/* Overlay pour mobile */}
      {isOpenMobile && (
        <div
          className="fixed inset-0 z-40 bg-neutral-900/60 backdrop-blur-xs md:hidden"
          onClick={onCloseMobile}
        />
      )}

      <aside
        className={`fixed top-0 bottom-0 left-0 z-50 flex w-64 flex-col border-r border-neutral-200 bg-white transition-all duration-200 ease-in-out dark:border-neutral-800 dark:bg-[#0A0A0A] md:static md:translate-x-0 ${
          isOpenMobile ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        {/* Brand Zone */}
        <div className="flex h-16 items-center justify-between border-b border-neutral-200 px-5 dark:border-neutral-800">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-neutral-900 text-white font-semibold text-sm tracking-tight shadow-xs dark:bg-white dark:text-neutral-900">
              P
            </div>
            <div>
              <span className="font-semibold text-base tracking-tight text-neutral-900 dark:text-neutral-100">
                PressingOS
              </span>
              <p className="text-[11px] text-neutral-500 font-normal dark:text-neutral-400">
                SaaS ERP · Côte d'Ivoire
              </p>
            </div>
          </div>
          <ThemeToggle />
        </div>

        {/* Tenant Context & Quick Switcher */}
        <div className="border-b border-neutral-100 bg-neutral-50/60 p-3.5 dark:border-neutral-800/80 dark:bg-[#111111]/60">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5 text-xs font-semibold text-neutral-900 dark:text-neutral-100 min-w-0">
              <Building2 className="h-3.5 w-3.5 text-neutral-500 dark:text-neutral-400 shrink-0" />
              <span className="truncate">{tenant ? tenant.name : 'SuperAdmin Globale'}</span>
            </div>
            <button
              onClick={onOpenLoginModal}
              title="Changer de rôle ou d'établissement"
              className="text-neutral-400 hover:text-neutral-900 dark:text-neutral-500 dark:hover:text-neutral-200 p-1 rounded hover:bg-neutral-200/50 dark:hover:bg-neutral-800 transition-colors"
            >
              <ArrowRightLeft className="h-3.5 w-3.5" />
            </button>
          </div>
          <div className="mt-1 flex items-center justify-between text-[11px] text-neutral-500 dark:text-neutral-400">
            <span>{tenant?.city || 'Côte d’Ivoire'}</span>
            <span className="font-mono text-neutral-700 dark:text-neutral-300 font-medium uppercase text-[10px] bg-neutral-200/60 dark:bg-neutral-800 px-1 rounded">
              {currentUser.role}
            </span>
          </div>
        </div>

        {/* Navigation list */}
        <nav className="flex-1 space-y-1 overflow-y-auto p-3">
          <div className="px-2 py-1 text-[11px] font-medium tracking-wider uppercase text-neutral-400 dark:text-neutral-500">
            {currentUser.role === 'superadmin' ? 'Administration SaaS' : 'Atelier & Gestion'}
          </div>
          {visibleNavItems.map((item) => {
            const Icon = item.icon;
            const isActive = currentView === item.id;
            return (
              <button
                key={item.id}
                onClick={() => handleNavClick(item.id)}
                className={`group flex w-full items-center justify-between rounded-md px-2.5 py-2 text-xs font-medium transition-colors ${
                  isActive
                    ? 'bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-950 font-semibold shadow-2xs'
                    : 'text-neutral-600 hover:bg-neutral-100 hover:text-neutral-900 dark:text-neutral-400 dark:hover:bg-neutral-900 dark:hover:text-neutral-100'
                }`}
              >
                <div className="flex items-center gap-2.5 min-w-0">
                  <Icon
                    className={`h-4 w-4 shrink-0 transition-colors ${
                      isActive
                        ? 'text-white dark:text-neutral-950'
                        : 'text-neutral-400 group-hover:text-neutral-700 dark:text-neutral-500 dark:group-hover:text-neutral-300'
                    }`}
                  />
                  <span className="truncate">{item.label}</span>
                </div>
                {item.badge && (
                  <span
                    className={`font-mono text-[10px] tabular-nums px-1.5 py-0.5 rounded ${
                      isActive
                        ? 'bg-neutral-800 text-neutral-200 dark:bg-neutral-200 dark:text-neutral-900'
                        : 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-400'
                    }`}
                  >
                    {item.badge}
                  </span>
                )}
              </button>
            );
          })}
        </nav>

        {/* User Card & Switch button */}
        <div className="border-t border-neutral-200 p-3 bg-neutral-50/50 dark:border-neutral-800 dark:bg-[#0D0D0D]">
          <button
            onClick={onOpenLoginModal}
            className="w-full flex items-center justify-between rounded-md p-1.5 hover:bg-neutral-100 dark:hover:bg-neutral-900 transition-colors text-left"
          >
            <div className="flex items-center gap-2 min-w-0">
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-neutral-800 text-white text-[11px] font-semibold shrink-0 dark:bg-neutral-200 dark:text-neutral-900">
                {currentUser.full_name.charAt(0)}
              </div>
              <div className="min-w-0">
                <div className="text-xs font-semibold text-neutral-900 dark:text-neutral-100 truncate">
                  {currentUser.full_name}
                </div>
                <div className="text-[10px] text-neutral-500 dark:text-neutral-400 font-mono truncate">
                  {currentUser.email}
                </div>
              </div>
            </div>
            <ArrowRightLeft className="h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500 shrink-0 ml-1" />
          </button>
        </div>
      </aside>
    </>
  );
};
