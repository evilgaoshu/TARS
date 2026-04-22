import type { ReactNode } from 'react';
import { useDeferredValue, useMemo, useState } from 'react';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Link } from 'react-router-dom';
import type { ConnectorManifest } from '@/lib/api/types';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/hooks/useI18n';
import { ChevronRight, ChevronLeft, Fingerprint, Cpu, Settings2, ShieldCheck, Search, Wand2, Zap, CheckCircle2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { applyConnectorProtocolPreset } from '@/lib/connector-samples';

function getBaseUrlPlaceholder(protocol: string | undefined, fieldKey: string, fieldDefault?: string): string {
  if (fieldKey !== 'base_url') {
    switch (fieldKey) {
      case 'host':
        return fieldDefault || '192.168.3.100';
      case 'username':
        return fieldDefault || 'root';
      default:
        return fieldDefault || fieldKey;
    }
  }
  switch (protocol) {
    case 'victoriametrics_http':
      return 'http://127.0.0.1:8428';
    case 'victorialogs_http':
      return 'https://play-vmlogs.victoriametrics.com';
    case 'prometheus_http':
      return 'http://127.0.0.1:9090';
    case 'jumpserver_api':
      return 'https://jumpserver.example.com';
    case 'ssh_native':
    case 'ssh_shell':
      return '';
    default:
      return fieldDefault || 'http://localhost:8080';
  }
}

function fieldHint(field: ConnectionField, t: ReturnType<typeof useI18n>['t']): string | undefined {
  if (field.secret) {
    return t('connectors.editor.secretHint');
  }
  if (field.key === 'credential_id') {
    return field.description || 'Create the SSH custody item in Ops > Secrets first, then paste its ID here';
  }
  return field.description;
}

function validateBaseUrl(url: string): { valid: boolean; error?: string } {
  if (!url || url.trim() === '') {
    return { valid: false, error: 'Base URL is required' };
  }
  const trimmed = url.trim();
  if (!trimmed.startsWith('http://') && !trimmed.startsWith('https://')) {
    return { valid: false, error: 'Base URL must start with http:// or https://' };
  }
  try {
    new URL(trimmed);
    return { valid: true };
  } catch {
    return { valid: false, error: 'Invalid URL format' };
  }
}

type TestStatus = {
  status: 'idle' | 'success' | 'error';
  summary?: string;
};

type ValidationError = {
  field: string;
  message: string;
};

type ConnectionField = NonNullable<ConnectorManifest['spec']['connection_form']>[number];

function formatTestStatusMessage(
  testStatus: TestStatus,
  t: ReturnType<typeof useI18n>['t'],
): string {
  const label = testStatus.status === 'success'
    ? t('connectors.editor.testSuccess')
    : t('connectors.editor.testFailed');
  const summary = testStatus.summary?.trim();
  if (!summary || summary === label) {
    return label;
  }
  return `${label}: ${summary}`;
}

type Props = {
  manifest: ConnectorManifest;
  disabled?: boolean;
  isEdit?: boolean;
  onTest?: () => void;
  testing?: boolean;
  onConfirm?: () => void;
  onCancel?: () => void;
  confirmLabel?: string;
  onChange: (manifest: ConnectorManifest) => void;
  createTemplates?: ConnectorManifest[];
  selectedCreateTemplate?: string | null;
  onSelectCreateTemplate?: (templateID: string) => void;
  testStatus?: TestStatus;
  validationErrors?: ValidationError[];
};

export function ConnectorManifestEditor({
  manifest,
  disabled = false,
  isEdit = false,
  onTest,
  testing = false,
  onConfirm,
  onCancel,
  confirmLabel,
  onChange,
  createTemplates = [],
  selectedCreateTemplate,
  onSelectCreateTemplate,
  testStatus = { status: 'idle' },
}: Props) {
  const { t } = useI18n();
  const createMode = !isEdit;
  const [currentStep, setCurrentStep] = useState(0);
  const [templateQuery, setTemplateQuery] = useState('');
  const [localValidationError, setLocalValidationError] = useState<string | null>(null);

  const deferredTemplateQuery = useDeferredValue(templateQuery);
  const connectionFields = manifest.spec.connection_form?.filter((field) => Boolean(field.key)) || [];
  const requiredFields = useMemo(
    () => manifest.spec.connection_form?.filter((field) => field.required && field.key) || [],
    [manifest.spec.connection_form],
  );
  const selectedTemplate = createTemplates.find((template) => template.metadata.id === selectedCreateTemplate);

  const filteredTemplates = useMemo(() => {
    const needle = deferredTemplateQuery.trim().toLowerCase();
    if (!needle) {
      return createTemplates;
    }
    return createTemplates.filter((template) => {
      const haystacks = [
        template.metadata.display_name,
        template.metadata.id,
        template.metadata.vendor,
        template.metadata.description,
        template.spec.type,
        template.spec.protocol,
      ];
      return haystacks.some((value) => (value || '').toLowerCase().includes(needle));
    });
  }, [createTemplates, deferredTemplateQuery]);

  const priorityTemplates = useMemo(() => filteredTemplates.filter((template) => isPriorityTemplate(template.metadata.id || '')), [filteredTemplates]);
  const secondaryTemplates = useMemo(() => filteredTemplates.filter((template) => !isPriorityTemplate(template.metadata.id || '')), [filteredTemplates]);

  const requiredFieldsComplete = useMemo(() => requiredFields.every((field) => {
    const key = field.key!;
    if (field.secret) {
      return Boolean(manifest.config?.secret_refs?.[key]);
    }
    return Boolean(manifest.config?.values?.[key]);
  }), [manifest, requiredFields]);

  const createReady = Boolean(selectedTemplate && manifest.metadata.id && manifest.spec.type && requiredFieldsComplete);

  const editSteps = useMemo(() => [
    { id: 'identity', title: t('connectors.editor.stepIdentity'), icon: <Fingerprint size={16} /> },
    { id: 'strategy', title: t('connectors.editor.stepStrategy'), icon: <Cpu size={16} /> },
    { id: 'config', title: t('connectors.editor.stepConfig'), icon: <Settings2 size={16} /> },
  ], [t]);

  const nextStep = () => setCurrentStep((step) => Math.min(step + 1, editSteps.length - 1));
  const prevStep = () => setCurrentStep((step) => Math.max(step - 1, 0));
  const isFinalStep = currentStep === editSteps.length - 1;

  const validateConnectionFields = useMemo(() => {
    const errors: ValidationError[] = [];
    for (const field of connectionFields) {
      const fieldKey = field.key!;
      if (fieldKey === 'base_url') {
        const value = manifest.config?.values?.[fieldKey] || '';
        const validation = validateBaseUrl(value);
        if (!validation.valid) {
          errors.push({ field: fieldKey, message: validation.error! });
        }
      }
      // Check other required fields
      if (field.required && fieldKey !== 'base_url') {
        const value = field.secret
          ? manifest.config?.secret_refs?.[fieldKey]
          : manifest.config?.values?.[fieldKey];
        if (!value || value.trim() === '') {
          errors.push({ field: fieldKey, message: `${field.label || fieldKey} is required` });
        }
      }
    }
    return errors;
  }, [connectionFields, manifest.config]);
  const canTest = validateConnectionFields.length === 0;

  const handleTest = () => {
    const errors = validateConnectionFields;
    if (errors.length > 0) {
      setLocalValidationError(errors.map(e => e.message).join(', '));
      return;
    }
    setLocalValidationError(null);
    onTest?.();
  };

  const updateMetadata = (key: keyof ConnectorManifest['metadata'], value: string) => {
    const nextMetadata = { ...manifest.metadata, [key]: value };
    if (key === 'id' && !nextMetadata.name) {
      nextMetadata.name = value;
    }
    onChange({ ...manifest, metadata: nextMetadata });
  };

  const updateSpec = (key: 'type' | 'protocol', value: string) => {
    if (key === 'protocol') {
      onChange(applyConnectorProtocolPreset({
        ...manifest,
        spec: {
          ...manifest.spec,
          protocol: value,
        },
      }, value));
      return;
    }
    onChange({ ...manifest, spec: { ...manifest.spec, [key]: value, import_export: manifest.spec.import_export } });
  };

  const updateMarketplace = (key: 'category' | 'source', value: string) => {
    onChange({ ...manifest, marketplace: { ...manifest.marketplace, [key]: value } });
  };

  const updateMarketplaceTags = (value: string) => {
    onChange({ ...manifest, marketplace: { ...manifest.marketplace, tags: splitValues(value) } });
  };

  const updateFieldValue = (fieldKey: string, value: string, secret: boolean) => {
    // Clear local validation error when user changes a field value
    if (localValidationError) {
      setLocalValidationError(null);
    }
    const nextValues = { ...(manifest.config?.values || {}) };
    const nextSecretRefs = { ...(manifest.config?.secret_refs || {}) };
    if (secret) {
      delete nextValues[fieldKey];
      nextSecretRefs[fieldKey] = value;
    } else {
      nextValues[fieldKey] = value;
      delete nextSecretRefs[fieldKey];
    }
    onChange({
      ...manifest,
      config: {
        values: nextValues,
        secret_refs: nextSecretRefs,
      },
    });
  };

  const editStepValid = useMemo(() => {
    if (currentStep === 0) {
      return Boolean(manifest.metadata.id);
    }
    if (currentStep === 1) {
      return Boolean(manifest.spec.type && manifest.spec.protocol);
    }
    if (currentStep === 2) {
      return requiredFieldsComplete;
    }
    return true;
  }, [currentStep, manifest.metadata.id, manifest.spec.protocol, manifest.spec.type, requiredFieldsComplete]);

  if (createMode) {
    return (
      <div className="flex flex-col gap-6 min-h-[450px]">
        <div className="rounded-xl border border-amber-500/10 bg-amber-500/5 p-4">
          <p className="m-0 text-sm text-amber-200/60">
            {t('connectors.editor.createHint')}
          </p>
        </div>

        <section className="space-y-4">
          <div className="space-y-1">
            <div className="text-[0.65rem] font-black uppercase tracking-[0.18em] text-muted-foreground">
              {t('connectors.editor.searchTitle')}
            </div>
            <p className="m-0 text-sm text-muted-foreground">
              {t('connectors.editor.searchDesc')}
            </p>
          </div>

          <div className="relative">
            <Search className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" size={16} />
            <Input
              value={templateQuery}
              onChange={(event) => setTemplateQuery(event.target.value)}
              placeholder={t('connectors.editor.searchPlaceholder')}
              className="h-11 border-white/10 bg-black/20 pl-10"
              disabled={disabled}
            />
          </div>

          <div className="rounded-2xl border border-white/10 bg-white/[0.03]">
            <div className="max-h-[260px] overflow-y-auto">
              {priorityTemplates.length ? (
                <div className="border-b border-white/5 px-3 py-2">
                  <div className="text-[0.6rem] font-black uppercase tracking-[0.22em] text-amber-300/80">
                    First-class
                  </div>
                </div>
              ) : null}
              <div className="divide-y divide-white/5">
                {priorityTemplates.map((template) => {
                  const active = selectedTemplate?.metadata.id === template.metadata.id;
                  return (
                    <TemplateRow
                      key={template.metadata.id}
                      template={template}
                      active={active}
                      disabled={disabled}
                      onSelect={onSelectCreateTemplate}
                    />
                  );
                })}
              </div>

              {secondaryTemplates.length ? (
                <>
                  <div className="border-y border-white/5 px-3 py-2">
                    <div className="text-[0.6rem] font-black uppercase tracking-[0.22em] text-muted-foreground">
                      Advanced
                    </div>
                  </div>
                  <div className="divide-y divide-white/5">
                    {secondaryTemplates.map((template) => {
                      const active = selectedTemplate?.metadata.id === template.metadata.id;
                      return (
                        <TemplateRow
                          key={template.metadata.id}
                          template={template}
                          active={active}
                          disabled={disabled}
                          onSelect={onSelectCreateTemplate}
                        />
                      );
                    })}
                  </div>
                </>
              ) : null}
            </div>
          </div>

          {!filteredTemplates.length ? (
            <div className="rounded-2xl border border-dashed border-white/10 bg-white/[0.02] p-8 text-center text-sm text-muted-foreground">
              {t('connectors.editor.noMatch')}
            </div>
          ) : null}
        </section>

        {selectedTemplate ? (
          <>
            <section className="space-y-4 rounded-2xl border border-white/5 bg-white/5 p-6 shadow-2xl">
              <div className="space-y-1">
                <div className="text-[0.65rem] font-black uppercase tracking-[0.18em] text-muted-foreground">
                  {t('connectors.editor.identityTitle')}
                </div>
                <p className="m-0 text-sm text-muted-foreground">
                  {t('connectors.editor.identityDesc')}
                </p>
              </div>

              <div className="flex flex-wrap gap-2">
                <TemplateBadge label={t('connectors.editor.type')} value={selectedTemplate.spec.type || '—'} />
                <TemplateBadge label={t('connectors.editor.protocol')} value={selectedTemplate.spec.protocol || '—'} />
                <TemplateBadge label={t('connectors.editor.vendor')} value={selectedTemplate.metadata.vendor || '—'} />
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <LabeledField
                  label={t('connectors.editor.idLabel')}
                  required
                  hint={t('connectors.editor.idHint')}
                >
                  <Input
                    value={manifest.metadata.id || ''}
                    onChange={(event) => updateMetadata('id', event.target.value)}
                    disabled={disabled}
                    placeholder={selectedTemplate.metadata.id || 'prometheus-main'}
                    className="font-mono border-white/10 focus-visible:ring-amber-500"
                  />
                </LabeledField>
                <LabeledField
                  label={t('connectors.editor.nameLabel')}
                  hint={t('connectors.editor.nameHint')}
                >
                  <Input
                    value={manifest.metadata.display_name || ''}
                    onChange={(event) => updateMetadata('display_name', event.target.value)}
                    disabled={disabled}
                    placeholder={selectedTemplate.metadata.display_name || ''}
                    className="border-white/10"
                  />
                </LabeledField>
              </div>

              <LabeledField
                label={t('connectors.editor.descLabel')}
                hint={t('connectors.editor.descHint')}
              >
                <textarea
                  value={manifest.metadata.description || ''}
                  onChange={(event) => updateMetadata('description', event.target.value)}
                  disabled={disabled}
                  placeholder={selectedTemplate.metadata.description || t('connectors.editor.descPlaceholder')}
                  className="min-h-[96px] w-full rounded-xl border border-white/10 bg-black/20 px-3 py-2 text-sm text-foreground shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber-500 disabled:cursor-not-allowed disabled:opacity-50"
                />
              </LabeledField>
            </section>

            <section className="space-y-4 rounded-2xl border border-success/20 bg-success/5 p-6">
              <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <div className="space-y-1">
                  <div className="text-[0.65rem] font-black uppercase tracking-[0.18em] text-success">
                    {t('connectors.editor.connectivityTitle')}
                  </div>
                  <p className="m-0 text-sm text-success/70">
                    {t('connectors.editor.connectivityDesc')}
                  </p>
                </div>
                {onTest ? (
                  <Button
                    type="button"
                    variant="outline"
                    className="h-9 rounded-full border-success px-6 text-[0.65rem] font-bold uppercase tracking-widest text-success hover:bg-success/10"
                    onClick={handleTest}
                    disabled={disabled || testing || !canTest}
                  >
                    {testing ? <div className="mr-2 h-3 w-3 animate-spin rounded-full border-2 border-success/30 border-t-success" /> : null}
                    {t('connectors.editor.testAction')}
                  </Button>
                ) : null}
              </div>

              {localValidationError ? (
                <div className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-xs text-destructive">
                  {`${t('connectors.editor.testFailed')}: ${localValidationError}`}
                </div>
              ) : testStatus.status !== 'idle' ? (
                <div className={cn(
                  'rounded-xl border px-4 py-3 text-xs',
                  testStatus.status === 'success'
                    ? 'border-success/30 bg-success/10 text-success'
                    : 'border-destructive/30 bg-destructive/10 text-destructive',
                )}>
                  {formatTestStatusMessage(testStatus, t)}
                </div>
              ) : (
                <div className="rounded-xl border border-white/10 bg-black/20 px-4 py-3 text-xs text-muted-foreground">
                  {t('connectors.editor.testIdle')}
                </div>
              )}

              {connectionFields.length ? (
                <div className="grid gap-6 md:grid-cols-2">
                  {connectionFields.map((field) => {
                    const fieldKey = field.key || '';
                    return (
                        <LabeledField
                          key={fieldKey}
                          label={field.label || fieldKey}
                          required={Boolean(field.required)}
                          hint={fieldHint(field, t)}
                        >
                          <div className="relative">
                            <Input
                            type="text"
                            className={cn(
                              'h-10 border-white/10 pr-10',
                              field.secret ? 'border-warning/20 bg-warning/5 font-mono focus-visible:ring-warning' : 'bg-black/20',
                            )}
                            value={field.secret ? manifest.config?.secret_refs?.[fieldKey] || '' : manifest.config?.values?.[fieldKey] || ''}
                            onChange={(event) => updateFieldValue(fieldKey, event.target.value, Boolean(field.secret))}
                            disabled={disabled}
                            placeholder={field.secret ? `secret_ref:${fieldKey}` : getBaseUrlPlaceholder(manifest.spec.protocol, fieldKey, field.default)}
                            />
                            {validateConnectionFields.some((error) => error.field === fieldKey) ? (
                              <div className="mt-2 text-[0.7rem] font-medium text-destructive">
                                {validateConnectionFields.find((error) => error.field === fieldKey)?.message}
                              </div>
                            ) : null}
                            {field.secret ? (
                              <Link
                                to="/ops?tab=secrets#secret-inventory"
                              className="absolute right-3 top-1/2 -translate-y-1/2 text-warning transition-colors hover:text-white"
                              title={t('common.edit')}
                            >
                              <Settings2 size={14} />
                            </Link>
                          ) : null}
                        </div>
                      </LabeledField>
                    );
                  })}
                </div>
              ) : (
                <div className="rounded-2xl border border-dashed border-white/10 bg-white/[0.02] p-8 text-center">
                  <div className="text-base font-bold text-foreground">{t('connectors.editor.noFields')}</div>
                  <p className="mt-2 text-xs text-muted-foreground">
                    {t('connectors.editor.noFieldsDesc')}
                  </p>
                </div>
              )}

              <LabeledField label={t('connectors.editor.afterSaveLabel')}>
                <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-white/5 bg-black/20 px-4 py-3 text-sm text-muted-foreground transition-colors hover:bg-black/30">
                  <Checkbox
                    checked={manifest.enabled ?? true}
                    onCheckedChange={(value) => onChange({ ...manifest, enabled: value === true })}
                    disabled={disabled}
                  />
                  <div className="space-y-0.5">
                    <span className="flex items-center gap-2 font-bold text-foreground">
                      <Zap size={14} className="text-amber-500" /> {t('connectors.editor.enableAfterSave')}
                    </span>
                    <p className="text-xs leading-5">
                      {t('connectors.editor.enableAfterSaveDesc')}
                    </p>
                  </div>
                </label>
              </LabeledField>
            </section>

            <div className="rounded-xl border border-dashed border-white/10 px-4 py-3 text-xs text-muted-foreground">
              {t('connectors.editor.opsHint')}
              <Link to="/ops?tab=connectors" className="text-amber-300 hover:text-amber-100">
                Ops
              </Link>
              {t('connectors.editor.opsSuffix')}
            </div>
          </>
        ) : (
          <div className="rounded-2xl border border-dashed border-white/10 bg-white/[0.02] p-10 text-center text-sm text-muted-foreground">
            {t('connectors.editor.chooseTypeHint')}
          </div>
        )}

        <div className="sticky bottom-0 z-10 flex items-center justify-between border-t border-white/5 bg-[var(--bg-surface-solid)]/95 pb-1 pt-6 backdrop-blur">
          <div className="flex items-center gap-2">
            {onCancel ? (
              <Button
                variant="ghost"
                onClick={onCancel}
                disabled={disabled}
                className="text-[0.65rem] font-bold uppercase tracking-widest opacity-40 hover:opacity-100"
              >
                {t('connectors.editor.cancel')}
              </Button>
            ) : null}
          </div>
          <Button
            variant="amber"
            onClick={onConfirm}
            disabled={disabled || !createReady}
            className="rounded-full px-8 text-[0.65rem] font-bold uppercase tracking-widest shadow-lg shadow-amber-500/20"
          >
            {confirmLabel || t('connectors.editor.createAction')} <Zap size={14} />
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-8 min-h-[450px]">
      <div className="flex items-center justify-between px-2">
        {editSteps.map((step, index) => (
          <div key={step.id} className="group flex flex-1 items-center last:flex-none">
            <div className={cn(
              'flex items-center gap-2 rounded-full px-3 py-1.5 transition-all duration-300',
              currentStep === index
                ? 'scale-105 border border-amber-500/30 bg-amber-500/20 text-amber-500'
                : currentStep > index
                  ? 'text-success opacity-80'
                  : 'text-text-muted opacity-40',
            )}>
              <div className={cn(
                'flex h-6 w-6 items-center justify-center rounded-full border-2 text-[0.6rem] font-black',
                currentStep === index ? 'border-amber-500 bg-amber-500 text-black' :
                currentStep > index ? 'border-success bg-success text-black' : 'border-white/10',
              )}>
                {currentStep > index ? '✓' : index + 1}
              </div>
              <div className="flex flex-col">
                <span className="text-[0.65rem] font-bold uppercase tracking-widest">{step.title}</span>
              </div>
            </div>
            {index < editSteps.length - 1 ? (
              <div className={cn(
                'mx-4 h-px flex-1 transition-colors duration-500',
                currentStep > index ? 'bg-success/30' : 'bg-white/5',
              )} />
            ) : null}
          </div>
        ))}
      </div>

      <div className="flex-1 animate-in fade-in slide-in-from-bottom-2 duration-500">
        {currentStep === 0 ? (
          <div className="space-y-6">
            <div className="mb-4 rounded-xl border border-amber-500/10 bg-amber-500/5 p-4">
              <p className="m-0 text-sm text-amber-200/60">
                {t('connectors.editor.identityTitleEdit')}
              </p>
            </div>
            <div className="grid gap-4 rounded-2xl border border-white/5 bg-white/5 p-6 shadow-2xl md:grid-cols-2">
              <LabeledField label={t('connectors.editor.idLabel')} required hint={isEdit ? t('common.na') : t('connectors.editor.idHint')}>
                <Input value={manifest.metadata.id || ''} onChange={(event) => updateMetadata('id', event.target.value)} disabled={disabled || isEdit} placeholder="victoriametrics-main" className={isEdit ? 'bg-white/5 font-mono opacity-60' : 'font-mono border-white/10 focus-visible:ring-amber-500'} />
              </LabeledField>
              <LabeledField label={t('connectors.editor.nameLabel')} hint={t('connectors.editor.nameHint')}>
                <Input value={manifest.metadata.display_name || ''} onChange={(event) => updateMetadata('display_name', event.target.value)} disabled={disabled} placeholder="VictoriaMetrics" className="border-white/10" />
              </LabeledField>
              <LabeledField label={t('connectors.editor.vendor')} hint="e.g. OpenSSH, VictoriaMetrics, Custom">
                <Input value={manifest.metadata.vendor || ''} onChange={(event) => updateMetadata('vendor', event.target.value)} disabled={disabled} placeholder="victoriametrics" className="border-white/10" />
              </LabeledField>
              <LabeledField label={t('connectors.editor.version')}>
                <Input value={manifest.metadata.version || ''} onChange={(event) => updateMetadata('version', event.target.value)} disabled={disabled} placeholder="1.0.0" className="border-white/10" />
              </LabeledField>
            </div>
          </div>
        ) : null}

        {currentStep === 1 ? (
          <div className="space-y-6">
            <div className="grid gap-6 rounded-2xl border border-white/5 bg-white/5 p-6 shadow-2xl">
              <div className="grid gap-6 md:grid-cols-2">
                <LabeledField label={t('connectors.editor.integrationType')} required hint="Category of this integration">
                  <select
                    className="flex h-10 w-full rounded-md border border-white/10 bg-black/20 px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber-500 disabled:cursor-not-allowed disabled:opacity-50"
                    value={manifest.spec.type || ''}
                    onChange={(event) => updateSpec('type', event.target.value)}
                    disabled={disabled}
                  >
                    <option value="">— select type —</option>
                    <option value="metrics">Metrics (Monitoring)</option>
                    <option value="logs">Logs (VictoriaLogs)</option>
                    <option value="execution">Execution (Action/Job)</option>
                    <option value="logging">Logging (Audit, legacy)</option>
                    <option value="security">Security (Scan)</option>
                  </select>
                </LabeledField>
                <LabeledField label={t('connectors.editor.protocol')} required hint="Specific driver protocol">
                  <select
                    className="flex h-10 w-full rounded-md border border-white/10 bg-black/20 px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber-500 disabled:cursor-not-allowed disabled:opacity-50"
                    value={manifest.spec.protocol || ''}
                    onChange={(event) => updateSpec('protocol', event.target.value)}
                    disabled={disabled}
                  >
                    <option value="">— select protocol —</option>
                    <option value="ssh_native">SSH Native</option>
                    <option value="victorialogs_http">VictoriaLogs HTTP</option>
                    <option value="prometheus_http">Prometheus HTTP</option>
                    <option value="victoriametrics_http">VictoriaMetrics HTTP</option>
                    <option value="jumpserver_api">JumpServer API</option>
                    <option value="mcp_generic">Generic MCP</option>
                    <option value="ssh_shell">SSH Shell (legacy)</option>
                  </select>
                </LabeledField>
              </div>

              <div className="grid gap-4 border-t border-white/5 pt-4 md:grid-cols-2">
                <LabeledField label={t('connectors.editor.category')}>
                  <Input value={manifest.marketplace.category || ''} onChange={(event) => updateMarketplace('category', event.target.value)} disabled={disabled} placeholder="observability" className="border-white/10" />
                </LabeledField>
                <LabeledField label={t('connectors.editor.tags')}>
                  <Input value={joinValues(manifest.marketplace.tags)} onChange={(event) => updateMarketplaceTags(event.target.value)} disabled={disabled} placeholder="metrics, cloud" className="border-white/10" />
                </LabeledField>
              </div>

              <div className="border-t border-white/5 pt-4">
                <LabeledField label={t('connectors.editor.visibility')}>
                  <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-white/5 bg-black/20 px-4 py-3 text-sm text-muted-foreground transition-colors hover:bg-black/30">
                    <Checkbox checked={manifest.enabled ?? true} onCheckedChange={(value) => onChange({ ...manifest, enabled: value === true })} disabled={disabled} />
                    <div className="space-y-0.5">
                      <span className="flex items-center gap-2 font-bold text-foreground">
                        <Zap size={14} className="text-amber-500" /> {t('connectors.editor.autoJoin')}
                      </span>
                      <p className="text-xs leading-5">{t('connectors.editor.autoJoinDesc')}</p>
                    </div>
                  </label>
                </LabeledField>
              </div>
            </div>
          </div>
        ) : null}

        {currentStep === 2 ? (
          <div className="space-y-6">
            {!connectionFields.length ? (
              <div className="flex flex-col items-center justify-center gap-4 rounded-3xl border border-dashed border-white/10 bg-white/[0.01] p-12 text-center">
                <div className="flex h-16 w-16 items-center justify-center rounded-full bg-white/5">
                  <ShieldCheck size={32} className="text-success opacity-40" />
                </div>
                <div className="space-y-1">
                  <h4 className="m-0 text-base font-bold text-text-primary">{t('connectors.editor.zeroConfig')}</h4>
                  <p className="text-xs text-text-muted">{t('connectors.editor.zeroConfigDesc')}</p>
                </div>
              </div>
            ) : (
              <div className="space-y-6">
                <div className="flex items-center justify-between rounded-2xl border border-success/20 bg-success/5 p-4">
                  <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-success/10">
                      <Settings2 size={20} className="text-success" />
                    </div>
                    <div>
                      <h4 className="m-0 text-sm font-bold uppercase tracking-widest text-success">{t('connectors.editor.connDetails')}</h4>
                      <p className="m-0 text-[0.65rem] text-success/60">{t('connectors.editor.connDetailsDesc')}</p>
                    </div>
                  </div>
                  {onTest ? (
                    <Button
                      type="button"
                      variant="outline"
                      className="h-9 rounded-full border-success px-6 text-[0.65rem] font-bold uppercase tracking-widest text-success hover:bg-success/10"
                      onClick={handleTest}
                      disabled={disabled || testing || !canTest}
                    >
                      {testing ? <div className="mr-2 h-3 w-3 animate-spin rounded-full border-2 border-success/30 border-t-success" /> : null}
                      {t('connectors.editor.testAction')}
                    </Button>
                  ) : null}
                </div>

                {localValidationError ? (
                  <div className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-xs text-destructive">
                    {localValidationError}
                  </div>
                ) : null}

                <div className="grid gap-6 rounded-2xl border border-white/5 bg-white/5 p-6 shadow-2xl md:grid-cols-2">
                  {connectionFields.map((field) => {
                    const fieldKey = field.key || '';
                    return (
                       <LabeledField key={fieldKey} label={field.label || fieldKey} required={Boolean(field.required)} hint={fieldHint(field, t)}>
                         <div className="relative">
                           <Input
                            type="text"
                            className={cn(
                              'h-10 border-white/10 pr-10',
                              field.secret ? 'border-warning/20 bg-warning/5 font-mono focus-visible:ring-warning' : 'bg-black/20',
                            )}
                            value={field.secret ? manifest.config?.secret_refs?.[fieldKey] || '' : manifest.config?.values?.[fieldKey] || ''}
                            onChange={(event) => updateFieldValue(fieldKey, event.target.value, Boolean(field.secret))}
                            disabled={disabled}
                            placeholder={field.secret ? `secret_ref:${fieldKey}` : getBaseUrlPlaceholder(manifest.spec.protocol, fieldKey, field.default)}
                          />
                          {validateConnectionFields.some((error) => error.field === fieldKey) ? (
                            <div className="mt-2 text-[0.7rem] font-medium text-destructive">
                              {validateConnectionFields.find((error) => error.field === fieldKey)?.message}
                            </div>
                          ) : null}
                          {field.secret && (manifest.spec.protocol === 'ssh_native' || manifest.spec.protocol === 'ssh_shell') && !manifest.config?.secret_refs?.[fieldKey]?.includes('/') && manifest.config?.secret_refs?.[fieldKey] !== '' && (
                            <div className="absolute -bottom-5 left-0 text-[0.6rem] text-rose-400 font-bold uppercase tracking-tighter animate-pulse">
                              Use secret_ref format (e.g. ssh/main/key)
                            </div>
                          )}
                          {field.secret ? (
                            <Link
                              to="/ops?tab=secrets#secret-inventory"
                              className="absolute right-3 top-1/2 -translate-y-1/2 text-warning transition-colors hover:text-white"
                              title={t('common.edit')}
                            >
                              <Settings2 size={14} />
                            </Link>
                          ) : null}
                        </div>
                      </LabeledField>
                    );
                  })}
                </div>
              </div>
            )}
          </div>
        ) : null}
      </div>

      <div className="sticky bottom-0 z-10 flex items-center justify-between border-t border-white/5 bg-[var(--bg-surface-solid)]/95 pb-1 pt-6 backdrop-blur">
        <div className="flex items-center gap-1">
          {onCancel ? (
            <Button
              variant="ghost"
              onClick={onCancel}
              disabled={disabled}
              className="text-[0.65rem] font-bold uppercase tracking-widest opacity-40 hover:opacity-100"
            >
              {t('connectors.editor.cancel')}
            </Button>
          ) : null}
          <Button
            variant="ghost"
            onClick={prevStep}
            disabled={currentStep === 0 || disabled}
            className="gap-2 text-[0.65rem] font-bold uppercase tracking-widest"
          >
            <ChevronLeft size={14} /> {t('connectors.editor.backStep')}
          </Button>
        </div>
        <div className="flex items-center gap-2">
          <span className="mr-4 text-[0.6rem] font-mono uppercase tracking-widest text-text-muted opacity-40">
            Step {currentStep + 1} of {editSteps.length}
          </span>
          {!isFinalStep ? (
            <Button
              variant="amber"
              onClick={nextStep}
              disabled={disabled || !editStepValid}
              className="gap-2 rounded-full px-8 text-[0.65rem] font-bold uppercase tracking-widest shadow-lg shadow-amber-500/20 transition-all disabled:grayscale disabled:opacity-20"
            >
              {t('connectors.editor.nextStep')} <ChevronRight size={14} />
            </Button>
          ) : (
            <Button
              variant="amber"
              onClick={onConfirm}
              disabled={disabled || !editStepValid}
              className="gap-2 rounded-full border-success bg-success px-8 text-[0.65rem] font-bold uppercase tracking-widest text-black shadow-lg shadow-amber-500/20 transition-all hover:bg-success/90 disabled:grayscale disabled:opacity-20"
            >
              {confirmLabel || t('connectors.editor.registerAction')} <Zap size={14} />
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

function TemplateRow({
  template,
  active,
  disabled,
  onSelect,
}: {
  template: ConnectorManifest;
  active: boolean;
  disabled: boolean;
  onSelect?: (templateID: string) => void;
}) {
  return (
    <button
      type="button"
      className={cn(
        'w-full px-4 py-3 text-left transition-colors',
        active ? 'bg-amber-500/10' : 'bg-transparent hover:bg-white/5',
      )}
      onClick={() => onSelect?.(template.metadata.id || '')}
      disabled={disabled}
      aria-pressed={active}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="text-sm font-bold text-foreground">{template.metadata.display_name || template.metadata.id}</div>
          <div className="mt-1 text-[0.65rem] uppercase tracking-[0.18em] text-muted-foreground">
            {template.spec.type} / {template.spec.protocol}
          </div>
          {active ? (
            <p className="mt-2 text-xs leading-5 text-muted-foreground">
              {template.metadata.description || '—'}
            </p>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          {isPriorityTemplate(template.metadata.id || '') ? (
            <span className="rounded-full border border-amber-500/20 bg-amber-500/10 px-2 py-1 text-[0.58rem] font-bold uppercase tracking-[0.18em] text-amber-300">
              Priority
            </span>
          ) : null}
          {active ? (
            <CheckCircle2 size={16} className="shrink-0 text-amber-400" />
          ) : (
            <Wand2 size={16} className="shrink-0 text-muted-foreground" />
          )}
        </div>
      </div>
    </button>
  );
}

function TemplateBadge({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-full border border-white/10 bg-black/20 px-3 py-1 text-[0.65rem] uppercase tracking-[0.18em] text-muted-foreground">
      {label}: <span className="font-bold text-foreground">{value}</span>
    </div>
  );
}

function LabeledField({ label, hint, required, children }: { label: string; hint?: string; required?: boolean; children: ReactNode }) {
  return (
    <label className="grid gap-2 text-sm">
      <span className="flex items-center gap-1 text-[0.68rem] font-bold uppercase tracking-[0.18em] text-muted-foreground">
        {label}
        {required ? <span className="font-black text-destructive">*</span> : null}
      </span>
      {children}
      {hint ? <span className="text-xs text-muted-foreground">{hint}</span> : null}
    </label>
  );
}

function splitValues(value?: string): string[] {
  return (value || '').split(',').map((item) => item.trim()).filter(Boolean);
}

function joinValues(values?: string[]): string {
  return (values || []).join(', ');
}

function isPriorityTemplate(templateID: string): boolean {
  return ['ssh-main', 'victoriametrics-main', 'victorialogs-main'].includes(templateID);
}
