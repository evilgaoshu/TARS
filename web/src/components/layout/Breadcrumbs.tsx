import React, { useMemo } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { ChevronRight, Home } from 'lucide-react'
import { useI18n } from '../../hooks/useI18n'
import { cn } from '../../lib/utils'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../ui/dropdown-menu'
import { useNavigationGroups, useFindRouteFunction, type NavGroupDefinition } from './navigation'

export const Breadcrumbs: React.FC = () => {
  const location = useLocation()
  const { t } = useI18n()
  const navigate = useNavigate()
  const groups = useNavigationGroups()
  const findNavigationRoute = useFindRouteFunction()

  const pathnames = useMemo(
    () => location.pathname.split('/').filter((x) => x),
    [location.pathname]
  )

  if (pathnames.length === 0) return null

  return (
    <nav className="flex items-center gap-1 mb-6 text-sm text-[var(--text-muted)] relative z-50">
      <Link
        to="/"
        className="text-inherit flex items-center p-1.5 rounded-md transition-all duration-200 hover:bg-white/5 hover:text-[var(--primary)]"
      >
        <Home size={16} />
      </Link>
      
        {pathnames.map((name, index) => {
        const routeTo = `/${pathnames.slice(0, index + 1).join('/')}`
        const isLast = index === pathnames.length - 1
        const route = findNavigationRoute(routeTo)
        const group = route ? groups.find((candidate) => candidate.routes.some((item) => item.id === route.id)) : undefined

        let label = name
        if (route) {
          label = t(route.labelKey as never)
        }

        return (
          <React.Fragment key={routeTo}>
            <ChevronRight size={14} className="opacity-30 mx-1" />
            
            {group ? (
              <BreadcrumbDropdown
                label={label}
                isLast={isLast}
                group={group}
                currentPath={location.pathname}
                navigate={navigate}
                t={t}
              />
            ) : (
              <span
                className={cn(
                  "px-2.5 py-1.5 rounded-lg text-inherit",
                  isLast && "text-[var(--text-primary)] font-bold"
                )}
              >
                {label}
              </span>
            )}
          </React.Fragment>
        )
      })}
    </nav>
  )
}

/** Radix-based breadcrumb dropdown for sibling route navigation */
const BreadcrumbDropdown: React.FC<{
  label: string;
  isLast: boolean;
  group: NavGroupDefinition;
  currentPath: string;
  navigate: ReturnType<typeof useNavigate>;
  t: ReturnType<typeof useI18n>['t'];
}> = ({ label, isLast, group, currentPath, navigate, t }) => (
  <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <button
        className={cn(
          "flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg",
          "bg-transparent border-none cursor-pointer text-inherit text-[inherit]",
          "transition-all duration-200 hover:bg-white/5 hover:text-[var(--primary)]",
          "data-[state=open]:bg-[var(--bg-surface-hover)]",
          isLast && "text-[var(--text-primary)] font-bold"
        )}
      >
        {label}
        <ChevronRight size={12} className="opacity-50 rotate-90" />
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" className="w-[220px]">
      {group.routes.map((r) => (
        <DropdownMenuItem
          key={r.path}
          onClick={() => navigate(r.path)}
          className={cn(
            "gap-2.5 cursor-pointer",
            currentPath === r.path && "bg-[var(--primary-glow)] text-[var(--primary)] font-semibold"
          )}
        >
          <span className="opacity-70"><r.icon size={14} /></span>
          {t(r.labelKey as never)}
        </DropdownMenuItem>
      ))}
    </DropdownMenuContent>
  </DropdownMenu>
)
