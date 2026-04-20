import { useState, useMemo } from 'react';
import { Link, useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  exportSkill,
  fetchSkill,
  setSkillEnabled,
  updateSkill,
  deleteSkill,
} from '../../lib/api/ops';
import { StatusBadge } from '@/components/ui/status-badge';
import {
  ArrowLeft,
  Power,
  Edit3,
  Download,
  Trash2,
  Eye,
  FileCode,
  File as FileIcon,
  FolderOpen,
  Save,
  X,
  Code2,
  AlertCircle,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card } from '@/components/ui/card';
import { Textarea } from '@/components/ui/textarea';
import { Input } from '@/components/ui/input';
import { TagInput } from '@/components/ui/tag-input';
import { useNotify } from '@/hooks/ui/useNotify';
import { useI18n } from '@/hooks/useI18n';
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
import type { SkillManifest } from '../../lib/api/types';


// ─── Component ──────────────────────────────────────────────────────────────

export const SkillDetailView = () => {
  const { id = '' } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const notify = useNotify();
  const { t } = useI18n();

  const [editMode, setEditMode] = useState(false);
  const [editDisplayName, setEditDisplayName] = useState('');
  const [editDescription, setEditDescription] = useState('');
  const [editTags, setEditTags] = useState<string[]>([]);
  const [editContent, setEditContent] = useState('');
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [saving, setSaving] = useState(false);

  const { data: skill, isLoading, error } = useQuery({
    queryKey: ['skill', id],
    queryFn: () => fetchSkill(id),
    enabled: !!id,
  });

  const skillMd = useMemo(() => {
    if (!skill) return '';
    const name = skill.metadata.display_name || skill.metadata.name || id;
    return `---\nname: ${name}\ndescription: ${skill.metadata.description || ''}\n---\n\n${skill.metadata.content || ''}\n`;
  }, [skill, id]);

  // ─── Mutations ────────────────────────────────────────────────────────

  const toggleMutation = useMutation({
    mutationFn: (enabled: boolean) => setSkillEnabled(id, enabled, `${enabled ? 'Enable' : 'Disable'} skill`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['skill', id] });
      notify.success(t('skills.statusUpdated', 'Skill status updated'));
    },
    onError: (err) => notify.error(err, t('skills.toggleFailed', 'Toggle failed')),
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteSkill(id, t('skills.deleteReason', 'Remove skill from registry')),
    onSuccess: () => {
      notify.success(t('skills.deleted', 'Skill deleted'));
      void navigate('/skills');
    },
    onError: (err) => notify.error(err, t('skills.deleteFailed', 'Delete failed')),
  });

  // ─── Actions ──────────────────────────────────────────────────────────

  const handleExportZip = async () => {
    if (!skill) return;
    try {
      const exported = await exportSkill(id, 'zip');
      const url = URL.createObjectURL(exported.content);
      const a = document.createElement('a');
      a.href = url; a.download = exported.filename; a.click();
      URL.revokeObjectURL(url);
      notify.success(t('skills.exported', 'Skill package exported'));
    } catch (e) {
      notify.error(e, t('skills.exportFailed', 'Export failed'));
    }
  };

  const handleExportYaml = async () => {
    try {
      const exported = await exportSkill(id, 'yaml');
      const url = URL.createObjectURL(exported.content);
      const a = document.createElement('a');
      a.href = url; a.download = exported.filename; a.click();
      URL.revokeObjectURL(url);
      notify.success(t('skills.exportedYaml', 'YAML exported'));
    } catch (e) {
      notify.error(e, t('skills.exportFailed', 'Export failed'));
    }
  };

  const startEdit = () => {
    if (!skill) return;
    setEditDisplayName(skill.metadata.display_name || skill.metadata.name || skill.metadata.id);
    setEditDescription(skill.metadata.description || '');
    setEditTags([...(skill.metadata.tags || [])]);
    setEditContent(skill.metadata.content || '');
    setEditMode(true);
  };

  const cancelEdit = () => {
    setEditMode(false);
  };

  const handleSave = async () => {
    if (!skill) return;
    try {
      setSaving(true);
      const updated: SkillManifest = {
        ...skill,
        metadata: {
          ...skill.metadata,
          display_name: editDisplayName.trim() || skill.metadata.display_name,
          description: editDescription.trim() || skill.metadata.description,
          tags: editTags.length > 0 ? editTags : undefined,
          content: editContent.trim() || undefined,
        },
      };
      await updateSkill(id, { operator_reason: t('skills.updateReason', 'Update skill via UI'), manifest: updated });
      queryClient.invalidateQueries({ queryKey: ['skill', id] });
      setEditMode(false);
      notify.success(t('skills.saved', 'Skill saved'));
    } catch (err) {
      notify.error(err, t('skills.saveFailed', 'Save failed'));
    } finally {
      setSaving(false);
    }
  };

  // ─── Loading / Error states ───────────────────────────────────────────

  if (isLoading) return (
    <div className="flex flex-col items-center justify-center min-h-[400px] text-muted-foreground gap-4">
      <div className="w-10 h-10 border-4 border-primary/20 border-t-primary rounded-full animate-spin" />
      <p className="animate-pulse">{t('common.loading', 'Loading skill…')}</p>
    </div>
  );

  if (!skill || error) return (
    <Card className="p-12 flex flex-col items-center gap-4">
      <FileCode size={48} className="text-muted-foreground/30" />
      <div className="text-xl font-bold text-foreground">{t('skills.detail.notFound', 'Skill not found')}</div>
      <p className="text-sm text-muted-foreground">{t('skills.detail.notFoundDesc', `The skill ${id} does not exist in the registry.`).replace('{{id}}', id)}</p>
      <Link to="/skills"><Button variant="outline">{t('skills.detail.back', 'Back to Registry')}</Button></Link>
    </Card>
  );

  const tags = skill.metadata.tags || [];
  const files = skill.files || [];
  const isDir = files.length > 0;

  return (
    <>
      <div className="animate-fade-in flex flex-col gap-6 max-w-4xl mx-auto">
        {/* ─── Header ────────────────────────────────────────────────── */}
        <div className="flex flex-col gap-4">
          <Link to="/skills" className="group flex items-center text-muted-foreground hover:text-primary text-sm transition-colors w-fit">
            <ArrowLeft size={14} className="mr-1.5 transition-transform group-hover:-translate-x-1" />
            {t('skills.detail.back', 'Back to Registry')}
          </Link>

          {error && (
            <div className="rounded-lg bg-red-500/10 p-4 border border-red-500/20 mb-2">
              <div className="flex items-center gap-2 text-red-500 mb-1">
                <AlertCircle size={18} />
                <span className="font-semibold text-sm">{t('skills.detail.operationFailed', 'Operation Failed')}</span>
                <span className="text-[0.6rem] ml-auto uppercase opacity-50">{t('common.copyErrorHint')}</span>
              </div>
              <p className="text-red-400 text-xs leading-relaxed select-text cursor-text bg-black/20 p-2 rounded font-mono">{error}</p>
            </div>
          )}

          <div className="flex flex-col md:flex-row md:items-start justify-between gap-4">
            <div className="space-y-3 min-w-0 flex-1">
              {editMode ? (
                <div className="space-y-3">
                  <div className="space-y-1">
                    <label className="text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-60">{t('common.name')}</label>
                    <Input
                      value={editDisplayName}
                      onChange={(e) => setEditDisplayName(e.target.value)}
                      className="text-2xl font-bold h-auto py-1 px-3"
                    />
                  </div>
                  <div className="space-y-1">
                    <label className="text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-60">{t('common.description')}</label>
                    <Input
                      value={editDescription}
                      onChange={(e) => setEditDescription(e.target.value)}
                      placeholder={t('common.description')}
                      className="text-sm"
                    />
                  </div>
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="flex items-center gap-3 flex-wrap">
                    <h1 className="text-3xl font-extrabold tracking-tight m-0 truncate">{skill.metadata.display_name || skill.metadata.name}</h1>
                    <div className="flex items-center gap-2">
                       <StatusBadge 
                         status={skill.enabled ? 'active' : 'disabled'} 
                         label={t(skill.enabled ? 'skills.status.active' : 'skills.status.disabled')}
                       />
                       <Badge variant="outline" className="text-[0.65rem] font-black uppercase tracking-widest py-0.5 border-primary/20 bg-primary/5 text-primary">
                         {t(`skills.source.${skill.metadata.source || 'custom'}`)}
                       </Badge>
                    </div>
                  </div>
                  
                  {skill.metadata.description ? (
                    <p className="text-sm text-muted-foreground m-0 max-w-2xl leading-relaxed">{skill.metadata.description}</p>
                  ) : null}

                  <div className="flex items-center gap-4 text-xs font-mono text-muted-foreground bg-white/[0.03] w-fit px-3 py-1.5 rounded-lg border border-white/[0.05]">
                    <div className="flex items-center gap-1.5">
                      <span className="opacity-50 tracking-tighter uppercase text-[0.6rem] font-black">ID:</span>
                      <span className="text-foreground/80">{skill.metadata.id}</span>
                    </div>
                    <div className="w-px h-3 bg-white/10" />
                    <div className="flex items-center gap-1.5">
                      <span className="opacity-50 tracking-tighter uppercase text-[0.6rem] font-black">apiVersion:</span>
                      <span className="text-foreground/80">{skill.api_version}</span>
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="flex flex-wrap gap-2 shrink-0 self-center">
              {editMode ? (
                <>
                  <Button variant="amber" size="sm" onClick={() => void handleSave()} disabled={saving}>
                    <Save size={14} /> {saving ? t('skills.saving', 'Saving…') : t('action.save', 'Save')}
                  </Button>
                  <Button variant="ghost" size="sm" onClick={cancelEdit} disabled={saving}>
                    <X size={14} /> {t('action.cancel', 'Cancel')}
                  </Button>
                </>
              ) : (
                <>
                  <Button variant="glass" size="sm" onClick={startEdit}>
                    <Edit3 size={14} /> {t('skills.detail.edit', 'Edit')}
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => toggleMutation.mutate(!(skill.enabled ?? false))} disabled={toggleMutation.isPending}>
                    <Power size={14} className="mr-1.5" /> {skill.enabled ? t('action.disable') : t('action.enable')}
                  </Button>

                  <div className="flex items-center border border-white/10 rounded-lg overflow-hidden shrink-0">
                    <Button variant="ghost" size="sm" className="rounded-none border-r border-white/10 hover:bg-white/5 h-8 gap-2" onClick={handleExportZip}>
                      <Download size={14} /> {t('skills.detail.exportPackage', 'Export Package')}
                    </Button>
                    <Button variant="ghost" size="sm" className="rounded-none hover:bg-white/5 h-8 w-8 p-0" title={t('skills.detail.exportYaml', 'Export K8s YAML')} onClick={handleExportYaml}>
                      <Code2 size={14} />
                    </Button>
                  </div>

                  <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive shrink-0" onClick={() => setDeleteOpen(true)}>
                    <Trash2 size={14} className="mr-1.5" /> {t('action.delete')}
                  </Button>
                </>
              )}
            </div>
          </div>

          {/* Tags */}
          {editMode ? (
            <div className="space-y-1">
              <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{t('common.tags', 'Tags')}</label>
              <TagInput value={editTags} onChange={setEditTags} placeholder={t('skills.tagsPlaceholder', 'Add tag (enter to confirm)…')} />
            </div>
          ) : tags.length > 0 ? (
            <div className="flex flex-wrap gap-1.5">
              {tags.map((t) => (
                <Badge key={t} variant="outline" className="text-xs px-2 py-0.5">{t}</Badge>
              ))}
            </div>
          ) : null}
        </div>

        {/* ─── File tree (directory skills) ──────────────────────────── */}
        {isDir ? (
          <Card className="p-0 overflow-hidden">
            <div className="px-5 py-3 border-b border-border bg-white/[0.02] flex items-center gap-2 text-sm font-semibold">
              <FolderOpen size={16} className="text-primary" />
              {t('skills.detail.directory', 'Skill Directory')}
              <span className="text-xs text-muted-foreground font-normal ml-auto">{files.length} {t('skills.detail.files', 'files')}</span>
            </div>
            <div className="divide-y divide-border/50">
              {files.map((f) => (
                <div key={f.path} className="flex items-center gap-3 px-5 py-2.5 hover:bg-white/[0.02] transition-colors">
                  {f.type === 'directory' ? (
                    <FolderOpen size={14} className="text-primary/60 shrink-0" />
                  ) : (
                    <FileIcon size={14} className="text-muted-foreground shrink-0" />
                  )}
                  <span className="font-mono text-sm text-foreground truncate">{f.path}</span>
                  {f.size != null ? (
                    <span className="text-xs text-muted-foreground ml-auto shrink-0">{formatSize(f.size)}</span>
                  ) : null}
                </div>
              ))}
            </div>
          </Card>
        ) : null}

        {/* ─── SKILL.md Content ──────────────────────────────────────── */}
        <Card className="p-0 overflow-hidden">
          <div className="px-5 py-3 border-b border-border bg-white/[0.02] flex items-center justify-between">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <FileCode size={16} className="text-primary" />
              SKILL.md
            </div>
            {!editMode ? (
              <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={startEdit}>
                <Edit3 size={12} /> {t('skills.detail.edit', 'Edit')}
              </Button>
            ) : (
              <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={cancelEdit}>
                <Eye size={12} /> {t('skills.detail.preview', 'Preview')}
              </Button>
            )}
          </div>

          {editMode ? (
            <div className="p-5">
              <Textarea
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                className="font-mono text-[0.8rem] leading-relaxed min-h-[400px] resize-y"
                spellCheck={false}
              />
              <p className="text-xs text-muted-foreground mt-2">
                {t('skills.detail.instructionsHint')}
              </p>
            </div>
          ) : (
            <div className="p-5">
              <pre className="text-[0.8rem] font-mono text-muted-foreground whitespace-pre-wrap leading-relaxed select-all m-0">
                {skill.metadata.content || skillMd}
              </pre>
            </div>
          )}
        </Card>
      </div>

      {/* ─── Delete Confirmation ───────────────────────────────────── */}
      <AlertDialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('skills.deleteTitle', 'Delete skill?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('skills.deleteDescription', 'Remove')} <strong>{skill.metadata.display_name || skill.metadata.id}</strong> {t('skills.deleteDescription2', 'from the registry. The source SKILL.md in Git is not affected.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleteMutation.isPending}>{t('common.cancel', 'Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => deleteMutation.mutate()}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? t('skills.deleting', 'Deleting…') : t('action.delete', 'Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
};

// ─── Helpers ────────────────────────────────────────────────────────────────

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
