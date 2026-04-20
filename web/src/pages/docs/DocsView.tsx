import React, { useMemo } from 'react';
import { useParams, Link, Navigate } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { 
  ChevronRight, 
  ShieldCheck, 
  FileText, 
  LifeBuoy,
  ExternalLink,
  Tag,
  Monitor,
  Terminal,
  Info,
  Cpu
} from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { useI18n } from '../../hooks/useI18n';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import yaml from 'js-yaml';

// OpenAPI Spec - Keep as raw for now as it's the core development ref
import tarsOpenApiYaml from '../../../../api/openapi/tars-mvp.yaml?raw';

// Bulk import all markdown files as raw strings using Vite's glob import
const allDocs = import.meta.glob('../../../../docs/**/*.md', { query: '?raw', import: 'default', eager: true });

interface DocItem {
  id: string;
  title: string;
  content: string;
  icon: React.ReactElement<LucideIcon>;
  category: string;
  lastUpdated: string;
  tarsVersion: string;
  isSwagger?: boolean;
}

interface OpenAPIDocument {
  openapi?: string;
  info?: {
    title?: string;
    version?: string;
    description?: string;
  };
  servers?: Array<{ url?: string; description?: string }>;
  paths?: Record<string, Record<string, OpenAPIOperation | unknown>>;
}

interface OpenAPIOperation {
  summary?: string;
  description?: string;
  operationId?: string;
  tags?: string[];
  responses?: Record<string, { description?: string } | unknown>;
}

const GET_DOCS_MAP = (lang: string): Record<string, DocItem> => {
  const isZh = lang === 'zh-CN';
  const suffix = isZh ? '.zh-CN.md' : '.md';
  
  const getRaw = (path: string) => (allDocs[path] as string) || '';

  return {
    'user-guide': { 
      id: 'user-guide', 
      title: isZh ? '用户指南' : 'User Guide', 
      content: getRaw(`../../../../docs/guides/user-guide${suffix}`), 
      icon: <FileText size={18} />, 
      category: 'Basics',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.2.0 (MVP)'
    },
    'web-console-guide': { 
      id: 'web-console-guide', 
      title: isZh ? 'Web Console 指南' : 'Web Console Guide', 
      content: getRaw(`../../../../docs/guides/web-console-guide${suffix}`), 
      icon: <Monitor size={18} />, 
      category: 'Basics',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.2.0'
    },
    'deployment-requirements': { 
      id: 'deployment-requirements', 
      title: isZh ? '部署要求' : 'Deployment Requirements', 
      content: getRaw(`../../../../docs/reference/deployment-requirements${suffix}`), 
      icon: <Cpu size={18} />, 
      category: 'Basics',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.2.0'
    },
    'admin-guide': { 
      id: 'admin-guide', 
      title: isZh ? '管理员指南' : 'Admin Guide', 
      content: getRaw(`../../../../docs/guides/admin-guide${suffix}`), 
      icon: <ShieldCheck size={18} />, 
      category: 'Administration',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.2.0'
    },
    'security-hardening': { 
      id: 'security-hardening', 
      title: isZh ? '安全加固' : 'Security Hardening', 
      content: getRaw(`../../../../docs/guides/security-hardening${suffix}`), 
      icon: <ShieldCheck size={18} />, 
      category: 'Administration',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.1.5'
    },
    'compatibility-matrix': { 
      id: 'compatibility-matrix', 
      title: isZh ? '兼容性矩阵' : 'Compatibility Matrix', 
      content: getRaw(`../../../../docs/reference/compatibility-matrix${suffix}`), 
      icon: <Tag size={18} />, 
      category: 'Administration',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.2.0'
    },
    'deployment-guide': { 
      id: 'deployment-guide', 
      title: isZh ? '部署手册' : 'Deployment Guide', 
      content: getRaw(`../../../../docs/guides/deployment-guide${suffix}`), 
      icon: <ExternalLink size={18} />, 
      category: 'Support',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.2.0'
    },
    'troubleshooting': { 
      id: 'troubleshooting', 
      title: isZh ? '故障排查' : 'Troubleshooting', 
      content: getRaw(`../../../../docs/guides/troubleshooting${suffix}`), 
      icon: <LifeBuoy size={18} />, 
      category: 'Support',
      lastUpdated: '2026-03-23',
      tarsVersion: 'Any'
    },
    'api-reference': { 
      id: 'api-reference', 
      title: isZh ? 'API 参考 (Swagger)' : 'API Reference (Swagger)', 
      content: tarsOpenApiYaml, 
      icon: <Terminal size={18} />, 
      category: 'Development',
      lastUpdated: '2026-03-23',
      tarsVersion: 'v1.2.0',
      isSwagger: true
    },
  };
};

const CATEGORY_LABELS: Record<string, Record<string, string>> = {
  'zh-CN': { Basics: '基础', Administration: '管理', Platform: '平台架构', Development: '开发参考', Support: '支持与排障' },
  'en-US': { Basics: 'Basics', Administration: 'Administration', Platform: 'Platform', Development: 'Development', Support: 'Support' },
};

export const DocsView = () => {
  const { slug } = useParams<{ slug: string }>();
  const { lang } = useI18n();

  const DOCS_MAP = useMemo(() => GET_DOCS_MAP(lang), [lang]);

  const apiSpec = useMemo(() => {
    try {
      return yaml.load(tarsOpenApiYaml) as OpenAPIDocument;
    } catch (e) {
      console.error('Failed to parse OpenAPI spec', e);
      return null;
    }
  }, []);

  if (!slug) {
    return <Navigate to="/docs/user-guide" replace />;
  }

  const activeDoc = DOCS_MAP[slug];
  if (!activeDoc) {
    return (
      <Card className="p-12 border-danger/20 flex flex-col items-center gap-4">
        <div className="text-xl font-bold text-danger">{lang === 'zh-CN' ? '未找到文档' : 'Document not found'}</div>
        <Link to="/docs/user-guide"><Button variant="outline">{lang === 'zh-CN' ? '返回用户指南' : 'Go to User Guide'}</Button></Link>
      </Card>
    );
  }

  return (
    <div className="animate-fade-in flex flex-col gap-6 w-full pb-20">
      <div className="flex flex-col lg:flex-row gap-8 items-start">
        {/* Sidebar Navigation */}
        <aside className="w-full lg:w-[280px] shrink-0 sticky top-6">
          <Card className="p-4 space-y-6">
            {Object.entries(CATEGORY_LABELS[lang] || CATEGORY_LABELS['en-US']).map(([cat, label]) => (
              <div key={cat} className="space-y-2">
                <h4 className="text-[0.65rem] font-black uppercase tracking-[0.2em] text-text-muted px-2">{label}</h4>
                <div className="flex flex-col gap-1">
                  {Object.values(DOCS_MAP).filter(d => d.category === cat).map(doc => (
                    <Link
                      key={doc.id}
                      to={`/docs/${doc.id}`}
                      className={clsx(
                        "flex items-center gap-3 p-2.5 rounded-xl transition-all duration-200 text-sm font-medium",
                        slug === doc.id 
                          ? "bg-primary text-black shadow-lg shadow-primary/20 scale-[1.02]" 
                          : "text-text-secondary hover:bg-white/5 hover:text-text-primary"
                      )}
                    >
                      <span className={clsx(slug === doc.id ? "text-black" : "text-primary opacity-70")}>
                        {doc.icon}
                      </span>
                      {doc.title}
                    </Link>
                  ))}
                </div>
              </div>
            ))}
          </Card>
        </aside>

        {/* Main Content Area */}
        <main className="flex-1 min-w-0 glass-card p-0 overflow-hidden border-white/10">
          {/* Doc Header */}
          <div className="p-8 border-b border-white/5 bg-white/[0.02]">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-primary/10 text-primary border border-primary/20">
                  {activeDoc.icon}
                </div>
                <h2 className="text-3xl font-black m-0">{activeDoc.title}</h2>
              </div>
              <div className="flex gap-2">
                <Badge variant="info" className="uppercase tracking-wider text-[0.68rem] font-bold">{activeDoc.tarsVersion}</Badge>
                  <Badge variant="muted" className="uppercase tracking-wider text-[0.68rem] font-bold">{lang === 'zh-CN' ? '更新于' : 'UPDATED'} {activeDoc.lastUpdated}</Badge>
              </div>
            </div>
          </div>

          <div className="p-8 lg:p-12">
            {activeDoc.isSwagger ? (
              <div className="swagger-wrapper rounded-2xl overflow-hidden border border-white/5">
                <OpenAPIReference spec={apiSpec} lang={lang} />
              </div>
            ) : (
              <div className="markdown-body">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                  {activeDoc.content}
                </ReactMarkdown>
              </div>
            )}
          </div>

          {/* Doc Footer */}
          <div className="p-8 border-t border-white/5 bg-white/[0.01] flex justify-between items-center">
              <div className="text-xs text-text-muted flex items-center gap-2">
                <Info size={14} /> 
               {lang === 'zh-CN' ? '发现文档错误？请提交 Issue 或 PR。' : 'Found a mistake? Submit an Issue or PR.'}
              </div>
            <Link to="/docs/user-guide" className="text-xs font-bold text-primary hover:underline flex items-center gap-1">
               {lang === 'zh-CN' ? '返回开头' : 'Back to Start'} <ChevronRight size={14} />
            </Link>
          </div>
        </main>
      </div>
    </div>
  );
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const clsx = (...classes: any[]) => classes.filter(Boolean).join(' ');

const HTTP_METHODS = ['get', 'post', 'put', 'patch', 'delete', 'head', 'options'] as const;

function OpenAPIReference({ spec, lang }: { spec: OpenAPIDocument | null; lang: string }) {
  const isZh = lang === 'zh-CN';
  const operations = useMemo(() => extractOperations(spec), [spec]);

  if (!spec) {
    return (
      <div className="p-8 text-sm text-danger">
        {isZh ? 'OpenAPI 文档解析失败。' : 'Failed to parse the OpenAPI document.'}
      </div>
    );
  }

  return (
    <div className="space-y-6 bg-black/20 p-6">
      <div className="rounded-2xl border border-white/10 bg-white/[0.03] p-5">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <div className="text-xs font-black uppercase tracking-[0.2em] text-text-muted">
              {isZh ? 'OpenAPI 参考' : 'OpenAPI Reference'}
            </div>
            <h3 className="mt-2 text-2xl font-black">{spec.info?.title || 'TARS API'}</h3>
            {spec.info?.description ? (
              <p className="mt-2 max-w-3xl text-sm leading-6 text-text-secondary">{spec.info.description}</p>
            ) : null}
          </div>
          <div className="flex flex-wrap gap-2">
            <Badge variant="info">{spec.openapi || 'OpenAPI'}</Badge>
            {spec.info?.version ? <Badge variant="muted">{spec.info.version}</Badge> : null}
            <Badge variant="success">{operations.length} {isZh ? '个接口' : 'operations'}</Badge>
          </div>
        </div>
        {spec.servers?.length ? (
          <div className="mt-4 grid gap-2">
            {spec.servers.map((server, index) => (
              <div key={`${server.url || 'server'}-${index}`} className="rounded-xl border border-white/10 bg-black/30 px-3 py-2 font-mono text-xs text-text-secondary">
                {server.url || '/'}{server.description ? <span className="ml-2 font-sans text-text-muted">{server.description}</span> : null}
              </div>
            ))}
          </div>
        ) : null}
      </div>

      <div className="space-y-3">
        {operations.map((operation) => (
          <div key={`${operation.method}-${operation.path}`} className="rounded-2xl border border-white/10 bg-white/[0.03] p-4">
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={methodBadgeVariant(operation.method)} className="font-mono uppercase">{operation.method}</Badge>
                  <code className="break-all rounded-lg bg-black/40 px-2 py-1 text-sm text-primary">{operation.path}</code>
                </div>
                <div className="mt-3 font-bold text-text-primary">{operation.summary || operation.operationId || (isZh ? '未命名接口' : 'Untitled operation')}</div>
                {operation.description ? (
                  <p className="mt-2 text-sm leading-6 text-text-secondary">{operation.description}</p>
                ) : null}
              </div>
              <div className="flex flex-wrap gap-2 md:justify-end">
                {operation.tags.slice(0, 3).map((tag) => <Badge key={tag} variant="muted">{tag}</Badge>)}
                {operation.responseCodes.slice(0, 4).map((code) => <Badge key={code} variant={code.startsWith('2') ? 'success' : 'warning'}>{code}</Badge>)}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function extractOperations(spec: OpenAPIDocument | null) {
  if (!spec?.paths) {
    return [];
  }

  return Object.entries(spec.paths).flatMap(([path, methods]) => (
    HTTP_METHODS.flatMap((method) => {
      const rawOperation = methods?.[method];
      if (!rawOperation || typeof rawOperation !== 'object') {
        return [];
      }
      const operation = rawOperation as OpenAPIOperation;
      return [{
        path,
        method,
        summary: operation.summary,
        description: operation.description,
        operationId: operation.operationId,
        tags: operation.tags || [],
        responseCodes: Object.keys(operation.responses || {}),
      }];
    })
  ));
}

function methodBadgeVariant(method: string): React.ComponentProps<typeof Badge>['variant'] {
  switch (method) {
    case 'get':
      return 'info';
    case 'post':
      return 'success';
    case 'put':
    case 'patch':
      return 'warning';
    case 'delete':
      return 'danger';
    default:
      return 'muted';
  }
}
