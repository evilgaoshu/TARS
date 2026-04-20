import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { BookOpen, Zap, Code2, Plus, Trash2, Upload, Download } from 'lucide-react';
import { PaginationControls } from '../../components/list/PaginationControls';
import { createSkill, deleteSkill, exportSkill, fetchSkills, getApiErrorMessage } from '../../lib/api/ops';
import type { SkillListResponse, SkillManifest } from '../../lib/api/types';
import { Button } from '../../components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { FilterBar } from '@/components/ui/filter-bar';
import { InlineStatus } from '@/components/ui/inline-status';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { StatusBadge } from '@/components/ui/status-badge';
import { GuidedFormDialog } from '@/components/operator/GuidedFormDialog';
import { useNotify } from '@/hooks/ui/useNotify';
import { useI18n } from '@/hooks/useI18n';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { TagInput } from '@/components/ui/tag-input';
import { LabeledField } from '@/components/ui/labeled-field';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';

// ─── SKILL.md default template ──────────────────────────────────────────────

const SKILL_MD_TEMPLATE = `1. Search knowledge base for relevant runbooks
2. Check current system metrics
3. Execute recommended remediation steps
`;

// ─── Filter Options ─────────────────────────────────────────────────────────

const statusOptions = [
  { value: '', label: 'All statuses' },
  { value: 'active', label: 'Active' },
  { value: 'draft', label: 'Draft' },
  { value: 'disabled', label: 'Disabled' },
  { value: 'deprecated', label: 'Deprecated' },
  { value: 'archived', label: 'Archived' },
];

// ─── Component ──────────────────────────────────────────────────────────────

export const SkillsList = () => {
  const navigate = useNavigate();
  const notify = useNotify();
  const { t } = useI18n();

  const [items, setItems] = useState<SkillManifest[]>([]);
  const [pageMeta, setPageMeta] = useState<Pick<SkillListResponse, 'page' | 'limit' | 'total' | 'has_next'>>({ page: 1, limit: 20, total: 0, has_next: false });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);

  // Create dialog state
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDescription, setNewDescription] = useState('');
  const [newTags, setNewTags] = useState<string[]>([]);
  const [newContent, setNewContent] = useState(SKILL_MD_TEMPLATE);
  const [nameError, setNameError] = useState('');

  // Delete dialog state
  const [deleteTarget, setDeleteTarget] = useState<SkillManifest | null>(null);
  const [deleting, setDeleting] = useState(false);

  // ─── Data loading ───────────────────────────────────────────────────────

  const loadData = async () => {
    try {
      setLoading(true);
      setError('');
      const response = await fetchSkills({ status: status || undefined, q: query || undefined, page, limit, sort_by: 'id', sort_order: 'asc' });
      setItems(response.items);
      setPageMeta({ page: response.page, limit: response.limit, total: response.total, has_next: response.has_next });
    } catch (loadError) {
      setError(getApiErrorMessage(loadError, t('skills.loadFailed', 'Failed to load skill registry.')));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    let active = true;
    void (async () => { if (active) await loadData(); })();
    return () => { active = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, status, page, limit]);

  // ─── Create ─────────────────────────────────────────────────────────────

  const resetCreateForm = () => {
    setNewName('');
    setNewDescription('');
    setNewTags([]);
    setNewContent(SKILL_MD_TEMPLATE);
    setNameError('');
  };

  const validateName = (name: string): boolean => {
    if (!name.trim()) { setNameError(t('skills.nameRequired', 'Name is required')); return false; }
    if (!/^[a-z][a-z0-9-]*[a-z0-9]$/.test(name.trim())) {
      setNameError(t('skills.nameInvalid', 'Lowercase letters, numbers and hyphens only. Must start with a letter.'));
      return false;
    }
    setNameError('');
    return true;
  };

  const handleCreate = async () => {
    const name = newName.trim();
    if (!validateName(name)) return;
    if (!newDescription.trim()) { notify.error(t('skills.descriptionRequired', 'Description is required')); return; }

    try {
      setCreating(true);
      const manifest: SkillManifest = {
        api_version: 'tars.skill/v1alpha1',
        kind: 'skill_package',
        enabled: false,
        metadata: {
          id: name,
          name: name,
          display_name: name.split('-').map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(' '),
          description: newDescription.trim(),
          tags: newTags.length > 0 ? newTags : undefined,
          content: newContent.trim() || undefined,
          source: 'custom',
        },
        spec: {},
      };
      const created = await createSkill({ operator_reason: t('skills.createReason', 'Create skill via UI'), manifest });
      setCreateOpen(false);
      resetCreateForm();
      notify.success(t('skills.created', 'Skill created'));
      void navigate(`/skills/${created.manifest.metadata.id}`);
    } catch (err) {
      notify.error(err, t('skills.createFailed', 'Failed to create skill'));
    } finally {
      setCreating(false);
    }
  };

  // ─── Import ─────────────────────────────────────────────────────────────


  // ─── Delete ─────────────────────────────────────────────────────────────

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      setDeleting(true);
      await deleteSkill(deleteTarget.metadata.id, t('skills.deleteReason', 'Remove skill from registry'));
      setDeleteTarget(null);
      notify.success(t('skills.deleted', `"${deleteTarget.metadata.display_name || deleteTarget.metadata.id}" deleted`));
      await loadData();
    } catch (err) {
      notify.error(err, t('skills.deleteFailed', 'Failed to delete skill'));
    } finally {
      setDeleting(false);
    }
  };

  // ─── Stats ──────────────────────────────────────────────────────────────

  const enabledCount = items.filter((i) => i.enabled).length;
  const customCount = items.filter((i) => (i.metadata.source) === 'custom' || (i.metadata.source) === 'imported').length;

  // ─── Helpers ────────────────────────────────────────────────────────────

  /** Merge all tags from metadata.tags and triggers into one badge array. */
  const getTags = (s: SkillManifest): string[] => {
    return [...(s.metadata.tags || [])];
  };

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <SectionTitle
          title={t('skills.title')}
          subtitle={t('skills.subtitle')}
        />
        <div className="flex gap-2">
          <Button variant="glass" size="sm" asChild>
            <Link to="/extensions">
              <Upload size={14} className="mr-1.5" /> {t('skills.import')}
            </Link>
          </Button>
          <Button variant="amber" size="sm" onClick={() => { resetCreateForm(); setCreateOpen(true); }}>
            <Plus size={14} className="mr-1.5" /> {t('skills.new')}
          </Button>
        </div>
      </div>

      {error ? <InlineStatus type="error" message={error} /> : null}

      <SummaryGrid>
        <StatCard 
          title={t('skills.stats.total')} 
          value={String(pageMeta.total)} 
          subtitle={t('skills.stats.totalDesc')} 
          icon={<BookOpen size={16} />} 
        />
        <StatCard 
          title={t('skills.stats.enabled')} 
          value={String(enabledCount)} 
          subtitle={t('skills.stats.enabledDesc')} 
          icon={<Zap size={16} />} 
        />
        <StatCard 
          title={t('skills.stats.source')} 
          value={String(customCount)} 
          subtitle={t('skills.stats.sourceDesc')} 
          icon={<Code2 size={16} />} 
        />
      </SummaryGrid>

      <FilterBar
        search={{ 
          value: query, 
          onChange: (v) => { setQuery(v); setPage(1); }, 
          placeholder: t('skills.search') 
        }}
        filters={[{
          key: 'status',
          value: status,
          onChange: (v) => { setStatus(v); setPage(1); },
          options: statusOptions.map(opt => ({ ...opt, label: opt.value ? t(`skills.status.${opt.value}`) : opt.label })),
        }]}
      />

      {/* Skills grid */}
      {loading ? (
        <div className="rounded-2xl border border-dashed border-border px-6 py-12 text-center text-sm text-muted-foreground animate-pulse">
          Loading skills…
        </div>
      ) : items.length === 0 ? (
        <Card className="flex flex-col items-center gap-4 p-12 text-center">
          <BookOpen size={40} className="text-muted-foreground/30" />
          <div className="text-lg font-semibold text-foreground">{t('skills.empty.title')}</div>
          <p className="text-sm text-muted-foreground max-w-sm">
            {t('skills.empty.description')}
          </p>
          <div className="flex gap-3">
            <Button variant="glass" size="sm" asChild>
              <Link to="/extensions">
                <Upload size={14} className="mr-1.5" /> {t('skills.import')}
              </Link>
            </Button>
            <Button variant="amber" size="sm" onClick={() => { resetCreateForm(); setCreateOpen(true); }}>
              <Plus size={14} className="mr-1.5" /> {t('skills.new')}
            </Button>
          </div>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {items.map((item) => {
            const tags = getTags(item);
            const isDir = (item.files?.length ?? 0) > 0;
            return (
              <Card key={item.metadata.id} className="group relative p-5 flex flex-col gap-3 hover:border-primary/30 transition-all">
                {/* Header */}
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <Link to={`/skills/${item.metadata.id}`} className="block text-base font-bold text-foreground hover:text-primary truncate">
                      {item.metadata.display_name || item.metadata.name || item.metadata.id}
                    </Link>
                    <div className="mt-0.5 flex items-center gap-2 font-mono text-xs text-muted-foreground">
                      <span className="truncate">{item.metadata.id}</span>
                      {isDir ? (
                         <Badge variant="outline" className="text-[0.55rem] px-1 py-0 shrink-0">{t('skills.directory', 'directory')}</Badge>
                      ) : null}
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-1.5">
                    <StatusBadge 
                      status={item.enabled ? 'active' : 'disabled'} 
                      label={t(item.enabled ? 'skills.status.active' : 'skills.status.disabled')}
                    />
                    <div className="text-[0.6rem] font-bold uppercase tracking-wider text-muted-foreground/60">
                      {t(`skills.source.${item.metadata.source || 'custom'}`)}
                    </div>
                  </div>
                </div>

                {/* Description */}
                {item.metadata.description ? (
                  <p className="text-sm text-muted-foreground line-clamp-2 m-0">{item.metadata.description}</p>
                ) : null}

                 {/* Tags */}
                {tags.length > 0 ? (
                  <div className="flex flex-wrap gap-1.5">
                    {tags.slice(0, 5).map((t) => (
                      <Badge key={t} variant="outline" className="text-[0.6rem] px-1.5 py-0">{t}</Badge>
                    ))}
                    {tags.length > 5 ? (
                      <Badge variant="outline" className="text-[0.6rem] px-1.5 py-0 text-muted-foreground">+{tags.length - 5}</Badge>
                    ) : null}
                  </div>
                ) : null}

                {/* Execution policy */}
                {item.spec?.governance?.execution_policy ? (
                  <div className="flex items-center gap-1.5">
                    <span className="text-xs text-muted-foreground">Policy:</span>
                    <Badge variant="secondary" className="text-[0.6rem] px-1.5 py-0">{item.spec.governance.execution_policy}</Badge>
                  </div>
                ) : null}

                {/* Actions — hover reveal */}
                <div className="flex items-center gap-2 pt-1 border-t border-border/50 opacity-0 group-hover:opacity-100 transition-opacity">
                  <Link to={`/skills/${item.metadata.id}`}>
                     <Button variant="ghost" size="sm" className="h-7 text-xs">{t('skills.view', 'View')}</Button>
                  </Link>
                  <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={() => void exportSkill(item.metadata.id, 'zip').then((exp) => {
                    const url = URL.createObjectURL(exp.content);
                    const a = document.createElement('a'); a.href = url; a.download = exp.filename; a.click(); URL.revokeObjectURL(url);
                    notify.success(t('skills.exported', 'Exported'));
                  }).catch((err) => notify.error(err, t('skills.exportFailed', 'Export failed')))}>
                    <Download size={11} /> {t('skills.export', 'Export')}
                  </Button>
                  <Button variant="ghost" size="sm" className="h-7 text-xs text-destructive hover:text-destructive ml-auto" onClick={() => setDeleteTarget(item)}>
                    <Trash2 size={11} /> {t('skills.delete', 'Delete')}
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {!loading && !error ? (
        <PaginationControls
          page={pageMeta.page}
          limit={pageMeta.limit}
          total={pageMeta.total}
          hasNext={pageMeta.has_next}
          onPageChange={setPage}
          onLimitChange={(next) => { setLimit(next); setPage(1); }}
        />
      ) : null}

      {/* ─── Create Skill Dialog ───────────────────────────────────────── */}
      <GuidedFormDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
         title={t('skills.createTitle', 'New Skill')}
         description={t('skills.createDescription', 'Create a SKILL.md — a playbook that teaches the AI how to handle a specific incident type.')}
         confirmLabel={creating ? t('skills.creating', 'Creating…') : t('skills.createConfirm', 'Create Skill')}
        loading={creating}
        onConfirm={() => void handleCreate()}
        wide
      >
        <div className="space-y-5">
          <div className="space-y-4">
              <LabeledField label={t('common.name', 'Name')} required>
              <Input
                value={newName}
                onChange={(e) => { setNewName(e.target.value); if (nameError) validateName(e.target.value); }}
                placeholder={t('skills.namePlaceholder', 'disk-cleanup')}
              />
              {nameError ? (
                <p className="text-xs text-destructive mt-1">{nameError}</p>
              ) : (
                  <p className="text-xs text-muted-foreground mt-1">{t('skills.nameHint', 'Lowercase kebab-case. This becomes the skill directory / file name.')}</p>
              )}
            </LabeledField>

              <LabeledField label={t('common.description', 'Description')} required>
              <Textarea
                rows={2}
                value={newDescription}
                onChange={(e) => setNewDescription(e.target.value)}
                placeholder={t('skills.descriptionPlaceholder', 'Diagnoses and resolves disk space alerts on Linux hosts…')}
              />
              <p className="text-xs text-muted-foreground mt-1">{t('skills.descriptionHint', 'The AI reads this to decide when to use this skill. Be specific.')}</p>
            </LabeledField>

            <LabeledField label="Tags">
              <TagInput
                value={newTags}
                onChange={setNewTags}
                placeholder="storage, linux, disk-usage-high…"
              />
              <p className="text-xs text-muted-foreground mt-1">Used for search and alert pre-filtering. Enter or comma to add.</p>
            </LabeledField>
          </div>

          <div className="space-y-2">
            <div className="text-xs font-black uppercase tracking-[0.18em] text-muted-foreground">{t('skills.instructions', 'Instructions (optional)')}</div>
            <Textarea
              rows={8}
              spellCheck={false}
              className="font-mono text-[0.8rem] leading-relaxed"
              placeholder={t('skills.instructionsPlaceholder', 'Step by step instructions for the AI agent…')}
              value={newContent}
              onChange={(e) => setNewContent(e.target.value)}
            />
            <p className="text-xs text-muted-foreground">{t('skills.instructionsHint', 'Markdown body of the SKILL.md file. You can edit this later.')}</p>
          </div>
        </div>
      </GuidedFormDialog>

      {/* ─── Delete Confirmation ───────────────────────────────────────── */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => { if (!open) setDeleteTarget(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('skills.deleteTitle', 'Delete skill?')}</AlertDialogTitle>
            <AlertDialogDescription>
               {t('skills.deleteDescription', 'Remove')} <strong>{deleteTarget?.metadata.display_name || deleteTarget?.metadata.id}</strong> {t('skills.deleteDescription2', 'from the registry. The source SKILL.md in Git is not affected.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
             <AlertDialogCancel disabled={deleting}>{t('common.cancel', 'Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => void handleDelete()}
              disabled={deleting}
            >
               {deleting ? t('skills.deleting', 'Deleting…') : t('skills.delete', 'Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
};
