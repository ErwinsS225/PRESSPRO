import React from 'react';
import { Sun, Moon } from 'lucide-react';
import { useTheme } from '../context/ThemeContext';

interface ThemeToggleProps {
  className?: string;
  showLabel?: boolean;
}

export const ThemeToggle: React.FC<ThemeToggleProps> = ({ className = '', showLabel = false }) => {
  const { theme, toggleTheme } = useTheme();
  const isDark = theme === 'dark';

  return (
    <button
      type="button"
      onClick={toggleTheme}
      aria-label={isDark ? 'Passer au mode clair' : 'Passer au mode sombre'}
      title={isDark ? 'Passer au mode clair' : 'Passer au mode sombre'}
      className={`group relative flex items-center gap-2 rounded-md p-1.5 text-neutral-600 transition-colors hover:bg-neutral-100 hover:text-neutral-900 dark:text-neutral-400 dark:hover:bg-neutral-800 dark:hover:text-neutral-100 ${className}`}
    >
      <div className="relative h-4 w-4">
        {isDark ? (
          <Sun className="h-4 w-4 transition-transform duration-200 rotate-0 scale-100 text-amber-400" />
        ) : (
          <Moon className="h-4 w-4 transition-transform duration-200 rotate-0 scale-100 text-neutral-700" />
        )}
      </div>
      {showLabel && <span className="text-xs font-medium">{isDark ? 'Mode clair' : 'Mode sombre'}</span>}
    </button>
  );
};
