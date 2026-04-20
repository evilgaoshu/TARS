/**
 * ExtensionsPage — master-detail split layout
 *
 * Left: candidate list (id / skill_name / status / review_state)
 * Right: selected candidate detail
 *   - Header + smart action buttons
 *   - Bundle info (skill metadata / governance / docs / tests)
 *   - Review history timeline
 *   - Validation report (errors / warnings)
 *   - Composer drawer (create new candidate)
 */

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import {
  CheckCircle2,
  ChevronRight,
  Clock,
  FileText,
  GitCommit,
  Package,
  ShieldCheck,
  TestTube2,
  XCircle,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { DetailHeader } from '@/components/ui/detail-header';
import { EmptyDetailState } from '@/components/ui/empty-detail-state';
import { FieldHint } from '@/components/ui/field-hint';
import { InlineStatus as StatusMessage } from '@/components/ui/inline-status';
import { LabeledField } from '@/components/ui/labeled-field';
import { SectionTitle, StatCard, SummaryGrid } from '@/components/ui/page-hero';
import { PanelCard } from '@/components/ui/panel-card';
import { RegistryCard, RegistryDetail, RegistryPanel, RegistrySidebar, SplitLayout } from '@/components/ui/registry-primitives';
import { Input } from '@/components/ui/input';
import { StatusBadge } from '@/components/ui/status-badge';
import { TagInput } from '@/components/ui/tag-input';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
import {
  createExtensionCandidate,
  fetchExtensions,
  getApiErrorMessage,
  importExtensionCandidate,
  reviewExtensionCandidate,
  validateExtensionCandidate,
  validateExtensionBundle,
} from '../../lib/api/ops';
import type { ExtensionBundle, ExtensionCandidate, SkillManifest } from '../../lib/api/types';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type FormState = {
  skillId: string;
  displayName: string;
  tags: string[];
  vendor: string;
  source: string;
  description: string;
  preferredTools: string;
  summary: string;
  stepsJson: string;
  docTitle: string;
  docSlug: string;
  docSummary: string;
  docContent: string;
  testName: string;
  testCommand: string;
};

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export const ExtensionsPage = () => {
  const [items, setItems] = useState<ExtensionCandidate[]>([]);
  const [totalItems, setTotalItems] = useState(0);
  const [loading, setLoading] = useState(true);
  const [globalError, setGlobalError] = useState('');
  const [actionError, setActionError] = useState('');
  const [actionSuccess, setActionSuccess] = useState('');
  const [busyAction, setBusyAction] = useState('');
  const [selectedID, setSelectedID] = useState<string>('');
  const [showComposer, setShowComposer] = useState(false);
  const [query, setQuery] = useState('');
  const [form, setForm] = useState<FormState>(() => defaultForm());
  const [previewResult, setPreviewResult] = useState<Awaited<ReturnType<typeof validateExtensionBundle>> | null>(null);

  const load = useCallback(async () => {
    try {
      setLoading(true);
      setGlobalError('');
      const response = await fetchExtensions({ q: query || undefined, page: 1, limit: 100, sort_by: 'updated_at', sort_order: 'desc' });
      setItems(response.items);
      setTotalItems(response.total);
      if (response.items.length > 0) {
        setSelectedID((current) => current || response.items[0].id);
      }
    } catch (err) {
      setGlobalError(getApiErrorMessage(err, 'Failed to load extension candidates.'));
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    void load();
  }, [load]);

  const selected = useMemo(() => items.find((i) => i.id === selectedID) ?? null, [items, selectedID]);

  const draftBundle = useMemo(() => buildBundle(form), [form]);

  const handleGenerate = async () => {
    try {
      setBusyAction('generate');
      setActionError('');
      setActionSuccess('');
      const created = await createExtensionCandidate({
        bundle: draftBundle,
        operator_reason: 'Generate governed skill bundle candidate',
      });
      setShowComposer(false);
      setForm(defaultForm());
      setPreviewResult(null);
      await load();
      setSelectedID(created.id);
      setActionSuccess(`Candidate ${created.id.slice(0, 8)} created.`);
    } catch (err) {
      setActionError(getApiErrorMessage(err, 'Failed to generate extension candidate.'));
    } finally {
      setBusyAction('');
    }
  };

  const handlePreviewValidate = async () => {
    try {
      setBusyAction('preview');
      setActionError('');
      const result = await validateExtensionBundle({ bundle: draftBundle });
      setPreviewResult(result);
    } catch (err) {
      setActionError(getApiErrorMessage(err, 'Failed to validate extension bundle.'));
    } finally {
      setBusyAction('');
    }
  };

  const handleValidate = async (candidateID: string) => {
    try {
      setBusyAction(`validate-${candidateID}`);
      setActionError('');
      await validateExtensionCandidate(candidateID);
      await load();
      setActionSuccess('Validation completed.');
    } catch (err) {
      setActionError(getApiErrorMessage(err, 'Failed to validate candidate.'));
    } finally {
      setBusyAction('');
    }
  };

  const handleImport = async (candidateID: string) => {
    try {
      setBusyAction(`import-${candidateID}`);
      setActionError('');
      await importExtensionCandidate(candidateID, 'Import reviewed extension candidate');
      await load();
      setActionSuccess('Candidate imported successfully.');
    } catch (err) {
      setActionError(getApiErrorMessage(err, 'Failed to import candidate.'));
    } finally {
      setBusyAction('');
    }
  };

  const handleReview = async (candidateID: string, reviewState: 'approved' | 'changes_requested' | 'rejected') => {
    const reasons: Record<string, string> = {
      approved: 'Approve reviewed extension candidate',
      changes_requested: 'Request changes for extension candidate',
      rejected: 'Reject extension candidate',
    };
    try {
      setBusyAction(`review-${candidateID}-${reviewState}`);
      setActionError('');
      await reviewExtensionCandidate(candidateID, {
        review_state: reviewState,
        operator_reason: reasons[reviewState],
      });
      await load();
      setActionSuccess(`Review state updated to ${reviewState}.`);
    } catch (err) {
      setActionError(getApiErrorMessage(err, 'Failed to update review state.'));
    } finally {
      setBusyAction('');
    }
  };

  const validatedCount = items.filter((i) => i.status === 'validated' || i.status === 'imported').length;
  const pendingReviewCount = items.filter((i) => i.review_state === 'pending').length;
  const importedCount = items.filter((i) => i.status === 'imported').length;

  return (
    <div className="animate-fade-in grid gap-6">
      <SectionTitle
        title="Extensions Center"
        subtitle="Generate governed skill bundle candidates and advance them through the validate → approve → import workflow into the skill registry."
      />

      <SummaryGrid>
        <StatCard title="Candidates" value={String(totalItems)} subtitle="Total candidates" />
        <StatCard title="Validated" value={String(validatedCount)} subtitle="Passed validation" />
        <StatCard title="Pending Review" value={String(pendingReviewCount)} subtitle="Awaiting approval" />
        <StatCard title="Imported" value={String(importedCount)} subtitle="Imported to skill registry" />
      </SummaryGrid>

      {actionSuccess ? <StatusMessage message={actionSuccess} type="success" /> : null}
      {actionError ? <StatusMessage message={actionError} type="error" /> : null}
      {globalError ? <StatusMessage message={globalError} type="error" /> : null}

      <SplitLayout
        className="items-start"
        sidebar={
          <RegistrySidebar className="p-5">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-lg font-semibold tracking-tight text-foreground">Candidates</h2>
              <Button variant="amber" className="h-8 px-3 text-xs" onClick={() => setShowComposer(true)}>
                + New
              </Button>
            </div>

            <div className="mt-4">
              <Input
                type="text"
                placeholder="Search..."
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                className="text-sm"
              />
            </div>

            <RegistryPanel className="mt-5" title="Candidate Registry" emptyText="No candidates yet.">
              {loading ? (
                <div className="rounded-2xl border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">
                  Loading…
                </div>
              ) : items.length === 0 ? (
                <div className="rounded-2xl border border-dashed border-border px-4 py-10 text-center text-sm text-muted-foreground">
                  No candidates yet.
                </div>
              ) : (
                <div className="grid max-h-[60vh] gap-2 overflow-y-auto pr-1">
                  {items.map((item) => {
                    const name = item.bundle.metadata.display_name || item.bundle.skill.metadata.display_name || item.bundle.skill.metadata.id;
                    const isSelected = item.id === selectedID;

                    return (
                      <button
                        key={item.id}
                        type="button"
                        onClick={() => {
                          setSelectedID(item.id);
                          setActionError('');
                          setActionSuccess('');
                        }}
                        className="block w-full appearance-none border-0 bg-transparent p-0 text-left"
                      >
                        <RegistryCard
                          active={isSelected}
                          title={name}
                          subtitle={item.id}
                          lines={[`Updated: ${item.updated_at ? new Date(item.updated_at).toLocaleString() : '—'}`]}
                          status={(
                            <div className="flex flex-wrap gap-1.5">
                              <WorkflowBadge value={item.status} kind="status" />
                              <WorkflowBadge value={item.review_state} kind="review" />
                            </div>
                          )}
                        />
                      </button>
                    );
                  })}
                </div>
              )}
            </RegistryPanel>
          </RegistrySidebar>
        }
        detail={selected ? (
          <RegistryDetail className="p-6">
            <CandidateDetail
              candidate={selected}
              busyAction={busyAction}
              onValidate={handleValidate}
              onReview={handleReview}
              onImport={handleImport}
            />
          </RegistryDetail>
        ) : (
          <EmptyDetailState
            title="Select a candidate"
            description="Choose an Extension Candidate from the left panel to view details, or click + New to create one."
          />
        )}
      />

      {showComposer ? (
        <PanelCard
          title="Candidate Composer"
          subtitle="Fill in the fields below and click Generate Candidate. The candidate will then enter the review workflow."
          headerAction={
            <Button variant="outline" className="h-8 px-3 text-xs" onClick={() => setShowComposer(false)}>
              Close
            </Button>
          }
        >
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <FormInput label="Skill ID" value={form.skillId} onChange={(v) => setForm((f) => ({ ...f, skillId: v }))} />
            <FormInput label="Display Name" value={form.displayName} onChange={(v) => setForm((f) => ({ ...f, displayName: v }))} />
            <LabeledField label="Tags">
              <TagInput value={form.tags} onChange={(v) => setForm((f) => ({ ...f, tags: v }))} placeholder="Add tag…" />
            </LabeledField>
            <FormInput label="Vendor" value={form.vendor} onChange={(v) => setForm((f) => ({ ...f, vendor: v }))} />
            <FormInput label="Source" value={form.source} onChange={(v) => setForm((f) => ({ ...f, source: v }))} />
          </div>

          <FormTextArea label="Description" rows={2} value={form.description} onChange={(v) => setForm((f) => ({ ...f, description: v }))} />

          <div className="grid gap-4 md:grid-cols-2">
            <FormInput label="Preferred Tools" value={form.preferredTools} onChange={(v) => setForm((f) => ({ ...f, preferredTools: v }))} />
          </div>

          <FormTextArea label="Planner Summary" rows={2} value={form.summary} onChange={(v) => setForm((f) => ({ ...f, summary: v }))} />
          <FormTextArea
            label="Planner Steps JSON"
            rows={8}
            value={form.stepsJson}
            onChange={(v) => setForm((f) => ({ ...f, stepsJson: v }))}
            hint="Must be valid JSON. Preview validation will use this content directly."
            className="font-mono"
          />

          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <FormInput label="Doc Title" value={form.docTitle} onChange={(v) => setForm((f) => ({ ...f, docTitle: v }))} />
            <FormInput label="Doc Slug" value={form.docSlug} onChange={(v) => setForm((f) => ({ ...f, docSlug: v }))} />
            <FormInput label="Doc Summary" value={form.docSummary} onChange={(v) => setForm((f) => ({ ...f, docSummary: v }))} />
          </div>

          <FormTextArea label="Doc Content" rows={4} value={form.docContent} onChange={(v) => setForm((f) => ({ ...f, docContent: v }))} />

          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            <FormInput label="Test Name" value={form.testName} onChange={(v) => setForm((f) => ({ ...f, testName: v }))} />
            <FormInput label="Test Command" value={form.testCommand} onChange={(v) => setForm((f) => ({ ...f, testCommand: v }))} />
          </div>

          <div className="flex flex-wrap justify-end gap-3">
            <Button variant="outline" onClick={() => void handlePreviewValidate()} disabled={busyAction !== ''}>
              {busyAction === 'preview' ? 'Validating…' : 'Preview Validation'}
            </Button>
            <Button variant="amber" onClick={() => void handleGenerate()} disabled={busyAction !== ''}>
              {busyAction === 'generate' ? 'Generating…' : 'Generate Candidate'}
            </Button>
          </div>

          {previewResult ? (
            <Card className="grid gap-3 p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-foreground">
                <CheckCircle2 className={cn('size-4', previewResult.validation.valid ? 'text-success' : 'text-danger')} />
                Preview Validation
              </div>
              <div className={cn('text-sm font-medium', previewResult.validation.valid ? 'text-success' : 'text-danger')}>
                {previewResult.validation.valid ? 'Validation passed' : 'Validation failed'}
              </div>
              {(previewResult.preview.summary || []).length > 0 ? (
                <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
                  {previewResult.preview.summary?.map((s) => <li key={s}>{s}</li>)}
                </ul>
              ) : null}
              {(previewResult.validation.warnings || []).length > 0 ? (
                <div className="text-sm text-warning">
                  Warnings: {previewResult.validation.warnings?.join(' | ')}
                </div>
              ) : null}
              {(previewResult.validation.errors || []).length > 0 ? (
                <div className="text-sm text-danger">
                  Errors: {previewResult.validation.errors?.join(' | ')}
                </div>
              ) : null}
            </Card>
          ) : null}
        </PanelCard>
      ) : null}
    </div>
  );
};

// ---------------------------------------------------------------------------
// CandidateDetail panel
// ---------------------------------------------------------------------------

function CandidateDetail({
  candidate,
  busyAction,
  onValidate,
  onReview,
  onImport,
}: {
  candidate: ExtensionCandidate;
  busyAction: string;
  onValidate: (id: string) => void;
  onReview: (id: string, state: 'approved' | 'changes_requested' | 'rejected') => void;
  onImport: (id: string) => void;
}) {
  const id = candidate.id;
  const name = candidate.bundle.metadata.display_name || candidate.bundle.skill.metadata.display_name || candidate.bundle.skill.metadata.id;
  const canImport = candidate.validation.valid && candidate.review_state === 'approved' && candidate.status !== 'imported';
  const isImported = candidate.status === 'imported';
  const busy = busyAction !== '';

  return (
    <div className="grid gap-5">
      <div className="grid gap-3">
        <DetailHeader
          title={name}
          status={(
            <div className="flex flex-wrap gap-2">
              <WorkflowBadge value={candidate.status} kind="status" />
              <WorkflowBadge value={candidate.review_state} kind="review" labelPrefix="review: " />
              {isImported ? <WorkflowBadge value="imported" kind="status" /> : null}
            </div>
          )}
          actions={(
            <>
              <Button
                variant="outline"
                disabled={busy || isImported}
                onClick={() => onValidate(id)}
                title="Run server-side validation"
              >
                {busyAction === `validate-${id}` ? 'Validating…' : 'Validate'}
              </Button>
              <Button
                variant="outline"
                disabled={busy || isImported}
                onClick={() => onReview(id, 'approved')}
                title={candidate.review_state === 'approved' ? 'Already approved' : 'Approve this candidate'}
              >
                {busyAction === `review-${id}-approved` ? 'Approving…' : 'Approve'}
              </Button>
              <Button
                variant="outline"
                disabled={busy || isImported}
                onClick={() => onReview(id, 'changes_requested')}
              >
                {busyAction === `review-${id}-changes_requested` ? 'Updating…' : 'Request Changes'}
              </Button>
              <Button
                variant="outline"
                disabled={busy || isImported}
                onClick={() => onReview(id, 'rejected')}
              >
                {busyAction === `review-${id}-rejected` ? 'Rejecting…' : 'Reject'}
              </Button>
              <Button
                variant="amber"
                disabled={busy || !canImport}
                title={!candidate.validation.valid ? 'Validation must pass first' : candidate.review_state !== 'approved' ? 'Approval required before import' : isImported ? 'Already imported' : 'Import to skill registry'}
                onClick={() => onImport(id)}
              >
                {busyAction === `import-${id}` ? 'Importing…' : isImported ? 'Imported ✓' : 'Import'}
              </Button>
            </>
          )}
        />

        <div className="font-mono text-xs text-muted-foreground">{id}</div>

        {candidate.imported_skill_id ? (
          <div className="rounded-xl border border-success/20 bg-success/10 px-4 py-3 text-sm text-success">
            Imported into skill{' '}
            <Link to={`/skills/${candidate.imported_skill_id}`} className="font-mono underline underline-offset-2">
              {candidate.imported_skill_id}
            </Link>
          </div>
        ) : null}
      </div>

      <div className="grid gap-5 xl:grid-cols-2">
        <DetailSection title="Skill Metadata" icon={<Package size={16} />}>
          <InfoRow label="ID" value={candidate.bundle.skill.metadata.id} mono />
          <InfoRow label="Tags" value={(candidate.bundle.skill.metadata.tags || []).join(', ') || '—'} />
          <InfoRow label="Vendor" value={candidate.bundle.skill.metadata.vendor || '—'} />
          <InfoRow label="Source" value={candidate.bundle.metadata.source || '—'} />
        </DetailSection>

        <DetailSection title="Governance Policy" icon={<ShieldCheck size={16} />}>
          <InfoRow label="Execution Policy" value={candidate.bundle.skill.spec?.governance?.execution_policy || '—'} />
          <InfoRow label="Read-only First" value={candidate.bundle.skill.spec?.governance?.read_only_first ? 'Yes' : 'No'} />
        </DetailSection>
      </div>

      <DetailSection title="Validation Report" icon={<CheckCircle2 size={16} />}>
        <div className="flex flex-wrap gap-2">
          <StatusBadge status={candidate.validation.valid ? 'valid' : 'invalid'} label={candidate.validation.valid ? 'Valid' : 'Invalid'} />
        </div>

        {(candidate.validation.errors || []).length > 0 ? (
          <div className="grid gap-2">
            {candidate.validation.errors?.map((e) => (
              <div key={e} className="flex items-start gap-2 text-sm text-danger">
                <XCircle className="mt-0.5 size-3.5 shrink-0" />
                <span>{e}</span>
              </div>
            ))}
          </div>
        ) : null}

        {(candidate.validation.warnings || []).length > 0 ? (
          <div className="grid gap-2">
            {candidate.validation.warnings?.map((w) => (
              <div key={w} className="flex items-start gap-2 text-sm text-warning">
                <Clock className="mt-0.5 size-3.5 shrink-0" />
                <span>{w}</span>
              </div>
            ))}
          </div>
        ) : null}

        {(candidate.validation.errors || []).length === 0 && (candidate.validation.warnings || []).length === 0 ? (
          <div className="text-sm text-muted-foreground">No errors or warnings.</div>
        ) : null}

        {(candidate.preview.summary || []).length > 0 ? (
          <div className="text-sm text-muted-foreground">
            Preview: {candidate.preview.summary?.join(' | ')}
          </div>
        ) : null}
      </DetailSection>

      <div className="grid gap-5 xl:grid-cols-2">
        {(candidate.bundle.docs || []).length > 0 ? (
          <DetailSection title={`Docs Assets (${candidate.bundle.docs?.length ?? 0})`} icon={<FileText size={16} />}>
            {candidate.bundle.docs?.map((doc) => (
              <div key={doc.slug || doc.id || doc.title} className="border-b border-border/60 pb-3 last:border-b-0 last:pb-0">
                <div className="text-sm font-semibold text-foreground">{doc.title || doc.slug}</div>
                {doc.summary ? <div className="mt-1 text-xs leading-5 text-muted-foreground">{doc.summary}</div> : null}
              </div>
            ))}
          </DetailSection>
        ) : null}

        {(candidate.bundle.tests || []).length > 0 ? (
          <DetailSection title={`Tests (${candidate.bundle.tests?.length ?? 0})`} icon={<TestTube2 size={16} />}>
            {candidate.bundle.tests?.map((test) => (
              <div key={test.id || test.name} className="border-b border-border/60 pb-3 last:border-b-0 last:pb-0">
                <div className="text-sm font-semibold text-foreground">{test.name || test.id}</div>
                {test.command ? <code className="mt-1 block font-mono text-xs text-muted-foreground">{test.command}</code> : null}
              </div>
            ))}
          </DetailSection>
        ) : null}
      </div>

      {(candidate.review_history || []).length > 0 ? (
        <DetailSection title="Review History" icon={<GitCommit size={16} />}>
          <div className="grid gap-3">
            {candidate.review_history?.slice().reverse().map((event, idx) => (
              <div key={`${event.state}-${event.created_at ?? idx}`} className="flex items-start gap-3">
                <div
                  className={cn(
                    'mt-1.5 size-2 shrink-0 rounded-full',
                    event.state === 'approved'
                      ? 'bg-success'
                      : event.state === 'rejected'
                        ? 'bg-danger'
                        : event.state === 'changes_requested'
                          ? 'bg-warning'
                          : 'bg-muted-foreground'
                  )}
                />
                <div className="flex-1 space-y-1.5">
                  <div className="flex flex-wrap items-center gap-2">
                    <WorkflowBadge value={event.state} kind="review" />
                    {event.created_at ? (
                      <span className="text-xs text-muted-foreground">{new Date(event.created_at).toLocaleString()}</span>
                    ) : null}
                  </div>
                  {event.reason ? <div className="text-sm italic text-muted-foreground">{event.reason}</div> : null}
                </div>
              </div>
            ))}
          </div>
        </DetailSection>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function DetailSection({ title, icon, children }: { title: string; icon?: ReactNode; children: ReactNode }) {
  return (
    <PanelCard
      title={title}
      icon={icon}
      className="h-full"
      headerAction={<ChevronRight size={14} className="text-muted-foreground" />}
    >
      {children}
    </PanelCard>
  );
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-3 text-sm">
      <span className="shrink-0 text-muted-foreground">{label}</span>
      <span className={cn('max-w-[60%] text-right text-foreground', mono && 'font-mono')}>
        {value}
      </span>
    </div>
  );
}

function WorkflowBadge({
  value,
  kind,
  labelPrefix,
}: {
  value?: string;
  kind: 'status' | 'review';
  labelPrefix?: string;
}) {
  const resolved = resolveWorkflowBadge(value, kind);
  return <StatusBadge status={resolved.status} label={`${labelPrefix ?? ''}${resolved.label}`} />;
}

function resolveWorkflowBadge(value: string | undefined, kind: 'status' | 'review') {
  const normalized = (value || 'unknown').toLowerCase();

  if (kind === 'status') {
    switch (normalized) {
      case 'validated':
        return { status: 'valid', label: 'validated' };
      case 'imported':
        return { status: 'success', label: 'imported' };
      case 'generated':
        return { status: 'warning', label: 'generated' };
      case 'invalid':
        return { status: 'invalid', label: 'invalid' };
      default:
        return { status: normalized, label: normalized.replace(/_/g, ' ') };
    }
  }

  switch (normalized) {
    case 'approved':
      return { status: 'approved', label: 'approved' };
    case 'changes_requested':
      return { status: 'warning', label: 'changes requested' };
    case 'rejected':
      return { status: 'rejected', label: 'rejected' };
    case 'imported':
      return { status: 'success', label: 'imported' };
    case 'pending':
      return { status: 'pending', label: 'pending' };
    default:
      return { status: normalized, label: normalized.replace(/_/g, ' ') };
  }
}

const FormInput = ({
  label,
  value,
  onChange,
  hint,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  hint?: string;
}) => (
  <LabeledField label={label}>
    <Input value={value} onChange={(e) => onChange(e.target.value)} />
    {hint ? <FieldHint>{hint}</FieldHint> : null}
  </LabeledField>
);

const FormTextArea = ({
  label,
  value,
  rows,
  onChange,
  hint,
  className,
}: {
  label: string;
  value: string;
  rows: number;
  onChange: (v: string) => void;
  hint?: string;
  className?: string;
}) => (
  <LabeledField label={label}>
    <Textarea rows={rows} value={value} onChange={(e) => onChange(e.target.value)} spellCheck={false} className={className} />
    {hint ? <FieldHint>{hint}</FieldHint> : null}
  </LabeledField>
);

// ---------------------------------------------------------------------------
// Helpers (kept from original)
// ---------------------------------------------------------------------------

function defaultForm(): FormState {
  return {
    skillId: 'disk-space-extension',
    displayName: 'Disk Space Extension',
    tags: ['incident-response', 'observability'],
    vendor: 'tars',
    source: 'official-generator',
    description: 'Investigate disk pressure before any write action is proposed.',
    preferredTools: 'knowledge.search, metrics.query_range',
    summary: 'Start with knowledge and metrics, then summarize safe next steps.',
    stepsJson: JSON.stringify([
      {
        id: 'step_1',
        tool: 'knowledge.search',
        required: true,
        reason: 'Load prior incident guidance.',
        params: { query: 'disk usage remediation' },
      },
      {
        id: 'step_2',
        tool: 'metrics.query_range',
        required: true,
        reason: 'Inspect disk saturation trends.',
        params: { query: 'node_filesystem_avail_bytes' },
      },
    ], null, 2),
    docTitle: 'Disk Space Extension Runbook',
    docSlug: 'disk-space-extension',
    docSummary: 'Operational guidance packaged with the bundle.',
    docContent: '# Disk Space Extension\n\n1. Validate affected mount points.\n2. Check inode pressure.\n3. Escalate before destructive cleanup.',
    testName: 'Bundle smoke validation',
    testCommand: 'go test ./...',
  };
}

function buildBundle(form: FormState): ExtensionBundle {
  const skillId = form.skillId.trim();
  const skillManifest: SkillManifest = {
    api_version: 'tars.skill/v1alpha1',
    kind: 'skill_package',
    enabled: false,
    metadata: {
      id: skillId,
      name: skillId,
      display_name: form.displayName.trim() || skillId,
      tags: form.tags,
      vendor: form.vendor.trim(),
      description: form.description.trim(),
      source: form.source.trim(),
      content: form.summary.trim(),
    },
    spec: {
      governance: {
        execution_policy: 'approval_first',
        read_only_first: true,
      },
    },
  };

  return {
    api_version: 'tars.extension/v1alpha1',
    kind: 'skill_bundle',
    metadata: {
      id: skillId,
      display_name: form.displayName.trim() || skillId,
      summary: form.description.trim(),
      source: form.source.trim() || 'official-generator',
      generated_by: 'extensions-center',
    },
    skill: skillManifest,
    docs: form.docTitle.trim() || form.docContent.trim() ? [{
      id: form.docSlug.trim() || skillId,
      slug: form.docSlug.trim() || skillId,
      title: form.docTitle.trim() || `${form.displayName.trim() || skillId} Runbook`,
      format: 'markdown',
      summary: form.docSummary.trim(),
      content: form.docContent.trim(),
    }] : [],
    tests: form.testName.trim() || form.testCommand.trim() ? [{
      id: 'bundle-smoke',
      name: form.testName.trim() || 'Bundle smoke validation',
      kind: 'smoke',
      command: form.testCommand.trim(),
    }] : [],
  };
}

export default ExtensionsPage;
