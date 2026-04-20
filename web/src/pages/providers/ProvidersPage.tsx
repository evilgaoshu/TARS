import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import {
  checkProviderAvailability,
  createProvider,
  fetchProvider as fetchProviderDetail,
  fetchProviderBindings,
  fetchProviders,
  listProviderModels,
  setProviderEnabled,
  updateProvider,
} from '../../lib/api/access';
import type {
  ProviderModelInfo,
  ProviderBindingsResponse,
  ProviderRegistryEntry,
} from '../../lib/api/types';
import { GuidedFormDialog } from '@/components/operator/GuidedFormDialog';
import { Card } from '@/components/ui/card';
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { NativeSelect } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { SectionTitle, SummaryGrid, StatCard } from '@/components/ui/page-hero';
import { PaginationControls } from '@/components/list/PaginationControls';
import { useNotify } from '@/hooks/ui/useNotify';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { 
  Plus, 
  Activity, 
  List, 
  Key, 
  ShieldCheck, 
  Search, 
  ChevronRight, 
  Zap,
  Globe,
  Settings2
} from 'lucide-react';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { useI18n } from '@/hooks/useI18n';
import { providerDiscoveryMessages } from '@/lib/ui/provider-discovery';

const buildProviderSchema = (t: (key: string, fallback?: string, vars?: Record<string, unknown>) => string) => z.object({
  id: z.string().min(1, t('prov.errors.nameRequired')),
  vendor: z.string().min(1, t('prov.errors.vendorRequired')),
  protocol: z.string().min(1, t('prov.errors.protocolRequired')),
  base_url: z.string().url(t('prov.errors.baseUrlInvalid')),
  api_key: z.string().optional(),
  api_key_ref: z.string().optional(),
  enabled: z.boolean(),
});

type ProviderSchema = ReturnType<typeof buildProviderSchema>;
type ProviderFormValues = z.infer<ProviderSchema>;

const VENDOR_DEFAULTS: Record<string, { protocol: string; base_url: string }> = {
  openai: { protocol: 'openai_compatible', base_url: 'https://api.openai.com/v1' },
  gemini: { protocol: 'gemini', base_url: 'https://generativelanguage.googleapis.com' },
  claude: { protocol: 'anthropic', base_url: 'https://api.anthropic.com' },
  openrouter: { protocol: 'openrouter', base_url: 'https://openrouter.ai/api/v1' },
  ollama: { protocol: 'ollama', base_url: 'http://localhost:11434' },
  lmstudio: { protocol: 'lmstudio', base_url: 'http://localhost:1234/v1' },
  dashscope: { protocol: 'openai_compatible', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  groq: { protocol: 'openai_compatible', base_url: 'https://api.groq.com/openai/v1' },
  deepseek: { protocol: 'openai_compatible', base_url: 'https://api.deepseek.com' },
  zhipu: { protocol: 'openai_compatible', base_url: 'https://open.bigmodel.cn/api/paas/v4' },
};

import { useCapabilities } from '../../lib/FeatureGateContext';
import { isEnabled, type Capability } from '../../lib/featureGates';

const providerVendorOptions: { value: string; label: string; capability?: Capability }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Google Gemini', capability: 'providers.gemini' },
  { value: 'dashscope', label: 'DashScope', capability: 'providers.dashscope' },
  { value: 'claude', label: 'Anthropic Claude' },
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'ollama', label: 'Ollama' },
  { value: 'lmstudio', label: 'LM Studio' },
  { value: 'groq', label: 'Groq' },
  { value: 'deepseek', label: 'DeepSeek' },
  { value: 'zhipu', label: 'Zhipu AI' },
];

const providerProtocolOptions = ['openai_compatible', 'anthropic', 'gemini', 'openrouter', 'ollama', 'lmstudio'];

export const ProvidersPage = () => {
  const { t } = useI18n();
  const providerSchema = useMemo(() => buildProviderSchema(t), [t]);
  const notify = useNotify();
  const [items, setItems] = useState<ProviderRegistryEntry[]>([]);
  const [pageMeta, setPageMeta] = useState({ page: 1, limit: 20, total: 0, has_next: false });
  const [bindings, setBindings] = useState<ProviderBindingsResponse | null>(null);
  
  const [activeID, setActiveID] = useState<string | 'new' | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [discoveredModels, setDiscoveredModels] = useState<ProviderModelInfo[]>([]);
  const [modelQuery, setModelQuery] = useState('');
  const [latencies, setLatencies] = useState<Record<string, number>>({});
  const [testingLatency, setTestingLatency] = useState<Record<string, boolean>>({});
  
  const [query, setQuery] = useState('');
  const [templatesText, setTemplatesText] = useState('');
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);

  const form = useForm<ProviderFormValues>({
    resolver: zodResolver(providerSchema),
    defaultValues: {
      id: '',
      vendor: 'openai',
      protocol: 'openai_compatible',
      base_url: '',
      api_key: '',
      api_key_ref: '',
      enabled: true,
    },
  });

  const { capabilities } = useCapabilities();
  const filteredVendorOptions = useMemo(() => {
    return providerVendorOptions.filter(o => !o.capability || isEnabled(capabilities, o.capability));
  }, [capabilities]);

  const loadDetail = useCallback(async (providerID: string) => {
    if (!providerID) return;
    try {
      const detail = await fetchProviderDetail(providerID);
      setActiveID(providerID);
      form.reset({
        id: detail.id,
        vendor: detail.vendor,
        protocol: detail.protocol,
        base_url: detail.base_url,
        api_key: '',
        api_key_ref: detail.api_key_ref || '',
        enabled: detail.enabled ?? true,
      });
      setTemplatesText(formatTemplates(detail.templates));
      setDiscoveredModels([]);
    } catch (error) {
      notify.error(error, t('prov.loadDetailFailed', 'Failed to load provider details'));
    }
  }, [form, notify, t]);

  const load = useCallback(async (preferredID?: string) => {
    try {
      setLoading(true);
      const [providersResp, providersConfig] = await Promise.all([
        fetchProviders({ q: query || undefined, page, limit }),
        fetchProviderBindings(),
      ]);
      setItems(providersResp.items || []);
      setPageMeta({ page: providersResp.page, limit: providersResp.limit, total: providersResp.total, has_next: providersResp.has_next });
      setBindings(providersConfig);
      if (preferredID) await loadDetail(preferredID);
    } catch (error) {
      notify.error(error, t('prov.loadFailed', 'Failed to load provider registry'));
    } finally {
      setLoading(false);
    }
  }, [limit, loadDetail, page, query, notify, t]);

  useEffect(() => { void load(); }, [load]);

  const filteredItems = useMemo(() => {
    const needle = (query || '').trim().toLowerCase();
    if (!needle) return items || [];
    return (items || []).filter((item) => {
      if (!item) return false;
      const searchFields = [item.id || '', item.vendor || '', item.protocol || '', item.base_url || ''];
      return searchFields.some((v) => v.toLowerCase().includes(needle));
    });
  }, [items, query]);

  const onSave = async (values: ProviderFormValues) => {
    try {
      setSaving(true);
      const payload: ProviderRegistryEntry = {
        ...values,
        api_key_set: false,
        templates: parseTemplates(templatesText),
      };
      const saved = activeID && activeID !== 'new'
        ? await updateProvider(activeID, payload)
        : await createProvider(payload);
      notify.success(activeID === 'new' ? t('prov.created', 'Provider created') : t('prov.updated', 'Provider updated'));
      setActiveID(null);
      await load(saved.id || values.id);
    } catch (error) {
      notify.error(error, t('prov.saveFailed', 'Failed to save provider'));
    } finally {
      setSaving(false);
    }
  };

  const handleToggleEnabled = async (id: string, current: boolean) => {
    try {
      await setProviderEnabled(id, !current);
      notify.success(!current ? t('prov.status.enabled', 'Provider enabled') : t('prov.status.disabled', 'Provider disabled'));
      await load();
    } catch (err) {
      notify.error(err, t('prov.status.toggleFailed', 'Failed to toggle status'));
    }
  };

  const handleLoadModels = async () => {
    const values = form.getValues();
    if (!values.id || !values.vendor || !values.base_url) {
      notify.warn(t('prov.discovery.missingFields', 'Missing required fields (ID, Vendor, or URL)'));
      return;
    }
    try {
      setModelsLoading(true);
      const entry: ProviderRegistryEntry = {
        ...values,
        api_key_set: false,
        templates: [],
      };
      const response = await listProviderModels(isNew ? undefined : activeID!, isNew ? entry : undefined);
      setDiscoveredModels(response.models || []);
      notify.success(providerDiscoveryMessages.discovered(t, response.models?.length || 0));
    } catch (err) {
      notify.error(err, t('prov.discovery.fetchFailed', 'Failed to fetch model list'));
    } finally {
      setModelsLoading(false);
    }
  };

  const handleSpeedTest = async (modelId: string) => {
    const values = form.getValues();
    try {
      setTestingLatency(prev => ({ ...prev, [modelId]: true }));
      const entry: ProviderRegistryEntry = {
        ...values,
        api_key_set: false,
        templates: [],
      };
      const start = Date.now();
      const response = await checkProviderAvailability(isNew ? undefined : activeID!, modelId, isNew ? entry : undefined);
      const end = Date.now();
      if (response.available) {
        setLatencies(prev => ({ ...prev, [modelId]: end - start }));
      } else {
        notify.warn(providerDiscoveryMessages.unreachable(t, modelId));
      }
    } catch {
      notify.error(null, t('prov.discovery.speedTestFailed', 'Speed test failed'));
    } finally {
      setTestingLatency(prev => ({ ...prev, [modelId]: false }));
    }
  };

  const filteredModels = useMemo(() => {
    const q = modelQuery.trim().toLowerCase();
    if (!q) return discoveredModels;
    return discoveredModels.filter(m => m.id.toLowerCase().includes(q));
  }, [discoveredModels, modelQuery]);

  const isNew = activeID === 'new';

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <SectionTitle title={t('prov.title')} subtitle={t('prov.description')} />
        <Button size="lg" className="rounded-2xl gap-2 h-12 px-6" onClick={() => {
          form.reset({ id: '', vendor: 'openai', protocol: 'openai_compatible', base_url: '', api_key: '', api_key_ref: '', enabled: true });
          setTemplatesText('');
          setDiscoveredModels([]);
          setActiveID('new');
        }}><Plus size={18} /> {t('prov.new')}</Button>
      </div>

      <SummaryGrid>
        <StatCard title={t('tech.provider')} value={String(pageMeta.total)} subtitle={t('prov.stats.totalDesc', 'Total registered')} icon={<Globe size={18} />} colorClass="text-sky-400" />
        <StatCard title={t('status.healthy')} value={String(items.filter(i => i.api_key_set || i.api_key_ref).length)} subtitle={t('prov.fields.apiKey')} icon={<Zap size={18} />} colorClass="text-emerald-400" />
        <StatCard title={t('status.enabled')} value={String(items.filter(i => i.enabled).length)} subtitle={t('prov.stats.enabledDesc', 'Active providers')} icon={<Activity size={18} />} colorClass="text-blue-400" />
        <StatCard title={t('prov.storage.title', 'Storage')} value={bindings?.configured ? t('prov.storage.persistent', 'Persistent') : t('prov.storage.memory', 'Memory')} subtitle={bindings?.path ? t('prov.storage.yaml', 'Yaml') : t('prov.storage.ephemeral', 'Ephemeral')} icon={<ShieldCheck size={18} />} colorClass="text-purple-400" />
      </SummaryGrid>

      <Card className="glass-card p-5 border-white/10 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="space-y-1">
          <div className="text-[0.65rem] font-black uppercase tracking-[0.18em] text-muted-foreground">{t('prov.handoff.label', 'Advanced / Raw')}</div>
          <p className="text-sm text-muted-foreground">
            {t('prov.handoff.description', 'Provider entries live here. Model binding is an advanced compatibility view; day-to-day role binding belongs with Agent Roles.')}
          </p>
        </div>
        <Button variant="outline" asChild className="rounded-xl">
          <Link to="/ops?tab=providers">{t('prov.handoff.action', 'Open Advanced Provider Ops')}</Link>
        </Button>
      </Card>

      <div className="space-y-4">
        <div className="flex items-center gap-4">
          <div className="relative max-w-xs w-full">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" size={16} />
            <Input placeholder={t('prov.search')} className="pl-10 h-10 rounded-xl" value={query} onChange={(e) => { setQuery(e.target.value); setPage(1); }} />
          </div>
        </div>

        {loading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6 animate-pulse">
            {[1, 2, 3, 4].map(i => <div key={i} className="h-48 rounded-2xl bg-white/5 border border-white/5" />)}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
            {filteredItems.map((item) => (
              <Card key={item.id} className="group glass-card-interactive p-6 flex flex-col gap-4 relative overflow-hidden">
                <div className="absolute top-4 right-4">
                  <Switch 
                    checked={item.enabled} 
                    onCheckedChange={() => void handleToggleEnabled(item.id || '', !!item.enabled)} 
                  />
                </div>
                
                <div className="flex flex-col gap-1">
                  <h3 className="text-xl font-bold tracking-tight pr-12 truncate" title={item.id}>{item.id}</h3>
                  <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono truncate">
                    <Globe size={10} /> {item.base_url}
                  </div>
                </div>

                <div className="flex flex-wrap gap-1.5 mt-auto">
                  <Badge variant="secondary" className="text-[0.6rem] uppercase tracking-wider font-black">{item.vendor}</Badge>
                  <Badge variant="outline" className="text-[0.6rem] uppercase tracking-wider">{item.protocol}</Badge>
                  {(item.api_key_set || item.api_key_ref) ? (
                    <Badge variant="outline" className="text-[0.6rem] bg-emerald-500/10 text-emerald-400 border-emerald-500/20">{t('prov.ready', 'READY')}</Badge>
                  ) : (
                    <Badge variant="outline" className="text-[0.6rem] bg-amber-500/10 text-amber-400 border-amber-500/20">{t('prov.noAuth', 'NO AUTH')}</Badge>
                  )}
                </div>

                <div className="flex items-center gap-2 pt-4 border-t border-white/5 mt-2 transition-all">
                  <Button variant="glass" size="sm" className="h-8 gap-2 w-full" onClick={() => loadDetail(item.id || '')}>
                    <Settings2 size={14} /> {t('prov.configure', 'Configure')}
                  </Button>
                </div>
              </Card>
            ))}
          </div>
        )}
        
        {!loading && (
          <PaginationControls
             page={pageMeta.page} limit={pageMeta.limit} total={pageMeta.total} hasNext={pageMeta.has_next}
             onPageChange={setPage} onLimitChange={(next) => { setLimit(next); setPage(1); }}
          />
        )}
      </div>

      <GuidedFormDialog
        open={!!activeID}
        onOpenChange={(open) => !open && setActiveID(null)}
        title={isNew ? t('prov.new') : `${t('action.edit')}: ${activeID}`}
        description={isNew ? t('prov.description') : t('prov.wizard.title')}
        onConfirm={form.handleSubmit(onSave)}
        confirmLabel={saving ? t('dash.hero.refreshing') : (isNew ? t('action.save') : t('status.success'))}
        loading={saving}
        wide
      >
        <Form {...form}>
          <div className="space-y-8 py-2">
            <div className="grid grid-cols-1 lg:grid-cols-[1fr_320px] gap-10">
              <div className="space-y-6">
                <div className="space-y-4">
                  <FormField control={form.control} name="id" render={({ field }) => (
                    <FormItem>
                       <FormLabel className="text-xs uppercase font-black tracking-widest text-muted-foreground mr-1">{t('prov.fields.name')}</FormLabel>
                      <span className="text-red-400 text-xs">*</span>
                       <FormControl><Input {...field} disabled={!isNew} placeholder={t('prov.fields.namePlaceholder', 'e.g. Aliyun Qwen')} className="h-12 text-lg font-bold rounded-xl" /></FormControl>
                       <FormDescription>{t('prov.fields.nameDescription', 'Friendly name to identify this provider.')}</FormDescription>
                      <FormMessage />
                    </FormItem>
                  )} />

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <FormField control={form.control} name="vendor" render={({ field }) => (
                      <FormItem className="flex flex-col">
                        <FormLabel className="text-xs uppercase font-black tracking-widest text-muted-foreground">{t('prov.fields.vendor')} <span className="text-red-400">*</span></FormLabel>
                        <Popover>
                          <PopoverTrigger asChild>
                            <FormControl>
                              <Button variant="outline" role="combobox" className={cn("w-full h-10 justify-between bg-white/5 border-white/10 rounded-xl", !field.value && "text-muted-foreground")}>
                                 {field.value ? field.value.toUpperCase() : t('prov.fields.vendorPlaceholder')}
                                <ChevronRight className="ml-2 h-4 w-4 shrink-0 rotate-90" />
                              </Button>
                            </FormControl>
                          </PopoverTrigger>
                          <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0">
                            <Command>
                               <CommandInput placeholder={t('prov.fields.vendorSearch', 'Search vendor...')} />
                              <CommandList>
                                 <CommandEmpty>{t('prov.fields.vendorEmpty', 'No vendor found.')}</CommandEmpty>
                                <CommandGroup>
                                  {filteredVendorOptions.map((v) => (
                                    <CommandItem
                                      value={v.value}
                                      key={v.value}
                                      onSelect={() => {
                                        form.setValue("vendor", v.value);
                                        const defaults = VENDOR_DEFAULTS[v.value];
                                        if (defaults) {
                                          form.setValue("protocol", defaults.protocol);
                                          form.setValue("base_url", defaults.base_url);
                                        }
                                      }}
                                    >
                                      {v.label}
                                    </CommandItem>
                                  ))}
                                </CommandGroup>
                              </CommandList>
                            </Command>
                          </PopoverContent>
                        </Popover>
                        <FormMessage />
                      </FormItem>
                    )} />

                    <FormField control={form.control} name="protocol" render={({ field }) => (
                      <FormItem>
                         <FormLabel className="text-xs uppercase font-black tracking-widest text-muted-foreground">{t('prov.fields.protocol')} <span className="text-red-400">*</span></FormLabel>
                        <FormControl>
                          <NativeSelect className="bg-white/5 h-10 rounded-xl" {...field}>
                             {providerProtocolOptions.map(o => <option key={o} value={o}>{o.replace('_', ' ').toUpperCase()}</option>)}
                          </NativeSelect>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )} />
                  </div>

                  <div className="p-4 rounded-2xl bg-white/5 border border-white/10 space-y-4">
                    <div className="flex items-center gap-2 text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground">
                      <Key size={12} /> {t('prov.sections.authentication')}
                    </div>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <FormField control={form.control} name="api_key_ref" render={({ field }) => (
                        <FormItem>
                       <FormLabel className="text-xs font-bold">{t('prov.fields.secretRef', 'Secret Reference')}</FormLabel>
                          <FormControl><Input {...field} placeholder="provider/openai/key" className="h-9 glass border-amber-500/30" /></FormControl>
                          <FormDescription className="text-[0.65rem]">{t('prov.fields.secretRefHint', 'Recommended: reference a secret stored in the platform.')}</FormDescription>
                        </FormItem>
                      )} />
                      <FormField control={form.control} name="api_key" render={({ field }) => (
                        <FormItem>
                       <FormLabel className="text-xs font-bold opacity-50">{t('prov.fields.rawApiKey', 'Raw API Key')}</FormLabel>
                          <FormControl><Input type="password" {...field} placeholder="••••••••••••" className="h-9 glass opacity-50 focus:opacity-100" /></FormControl>
                          <FormDescription className="text-[0.65rem] text-amber-500/60">{t('prov.fields.rawApiKeyWarning', 'Discouraged: plaintext key will be stored in registry if not using secret ref.')}</FormDescription>
                        </FormItem>
                      )} />
                    </div>
                  </div>

                  <FormField control={form.control} name="base_url" render={({ field }) => (
                    <FormItem>
                       <FormLabel className="text-xs uppercase font-black tracking-widest text-muted-foreground">{t('prov.fields.baseUrl')} <span className="text-red-400">*</span></FormLabel>
                      <FormControl><Input {...field} placeholder="https://..." className="h-10 glass font-mono text-sm" /></FormControl>
                      <FormMessage />
                    </FormItem>
                  )} />
                </div>

                <div className="pt-6 border-t border-white/5">
                   <div className="flex items-center justify-between mb-4">
                      <div className="flex flex-col gap-1">
                        <h4 className="text-sm font-bold flex items-center gap-2"><List size={14} /> {t('prov.discovery.title')}</h4>
                         <p className="text-xs text-muted-foreground">{t('prov.discovery.desc')}</p>
                      </div>
                      <Button type="button" variant="secondary" size="sm" className="h-8 rounded-lg" onClick={() => void handleLoadModels()} disabled={modelsLoading}>
                        {modelsLoading ? t('prov.discovery.fetching') : t('prov.discovery.fetch')}
                      </Button>
                   </div>

                   {discoveredModels.length > 0 && (
                     <div className="space-y-3 p-4 rounded-2xl bg-bg-muted border border-white/5">
                        <div className="relative">
                          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" size={14} />
                          <Input 
                            placeholder={t('prov.discovery.search')} 
                            className="pl-9 h-9 text-xs glass" 
                            value={modelQuery} 
                            onChange={(e) => setModelQuery(e.target.value)} 
                          />
                        </div>
                        <div className="max-h-[250px] overflow-y-auto pr-2 custom-scrollbar space-y-2">
                          {filteredModels.map(m => (
                            <div key={m.id} className="flex items-center justify-between p-2.5 rounded-xl bg-white/5 border border-white/5 hover:bg-white/10 transition-colors group">
                              <div className="flex items-center gap-3">
                                <Badge variant="outline" className="text-[0.55rem] uppercase tracking-wider">model</Badge>
                                <span className="text-xs font-bold font-mono">{m.id}</span>
                              </div>
                              <div className="flex items-center gap-2">
                                {latencies[m.id] && <span className="text-[0.6rem] font-bold text-emerald-400">{latencies[m.id]}ms</span>}
                                <Button 
                                  type="button" 
                                  variant="ghost" 
                                  size="icon" 
                                  className="h-6 w-6 text-muted-foreground hover:text-emerald-400"
                                  onClick={() => void handleSpeedTest(m.id)}
                                  disabled={testingLatency[m.id]}
                                >
                                  <Zap size={12} className={cn(testingLatency[m.id] && "animate-pulse")} />
                                </Button>
                              </div>
                            </div>
                          ))}
                        </div>
                     </div>
                   )}

                   {!discoveredModels.length && !modelsLoading && (
                      <div className="p-10 border border-dashed border-white/10 rounded-2xl flex flex-col items-center gap-3 text-muted-foreground">
                         <Search size={24} className="opacity-20" />
                         <span className="text-xs">{t('prov.discovery.empty', 'No models loaded. Use "Fetch Models" to discover.')}</span>
                      </div>
                    )}
                </div>
              </div>

              <div className="space-y-6">
                <div className="p-5 rounded-2xl bg-white/5 border border-white/5 space-y-4">
                  <div className="space-y-1">
                    <div className="text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-50">{t('prov.compatibility.label', 'Platform Defaults')}</div>
                    <div className="text-sm font-semibold text-foreground">{t('prov.compatibility.title', 'Read-only on this page')}</div>
                  </div>
                  <div className="text-xs text-muted-foreground">{t('prov.compatibility.description', 'Provider registry now owns connectivity and discovery. Platform primary/assist defaults stay visible here for context, but day-to-day model selection belongs in Agent Roles or Advanced Provider Ops.')}</div>
                  <div className="space-y-2 border-t border-white/5 pt-4 text-xs text-muted-foreground">
                    <div>{t('prov.compatibility.primary', 'Platform Primary')}: <span className="font-mono text-foreground">{formatPlatformBinding(bindings?.bindings.primary?.provider_id, bindings?.bindings.primary?.model)}</span></div>
                    <div>{t('prov.compatibility.assist', 'Platform Assist')}: <span className="font-mono text-foreground">{formatPlatformBinding(bindings?.bindings.assist?.provider_id, bindings?.bindings.assist?.model)}</span></div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" asChild className="rounded-xl">
                      <Link to="/identity/agent-roles">{t('prov.compatibility.agentRolesAction', 'Open Agent Roles')}</Link>
                    </Button>
                    <Button variant="outline" asChild className="rounded-xl">
                      <Link to="/ops?tab=providers">{t('prov.compatibility.advancedAction', 'Open Advanced Provider Ops')}</Link>
                    </Button>
                  </div>
                </div>

                <div className="p-5 rounded-2xl bg-white/5 border border-white/5 space-y-4">
                   <div className="text-[0.65rem] font-black uppercase tracking-widest text-muted-foreground opacity-50">{t('prov.override.title')}</div>
                   <div className="space-y-3">
                      <FormLabel className="text-[0.65rem]">{t('prov.override.advancedTemplates')}</FormLabel>
                      <Textarea 
                        className="font-mono text-[0.65rem] min-h-[120px] bg-black/20 border-white/5 resize-none" 
                        value={templatesText} 
                        onChange={(e) => setTemplatesText(e.target.value)} 
                         placeholder={t('prov.override.placeholder')} 
                       />
                   </div>
                </div>

                {!isNew && !bindings?.configured && (
                     <div className="p-4 bg-amber-500/5 border border-amber-500/20 rounded-2xl text-[0.6rem] leading-relaxed text-amber-100/60 font-medium">
                     <Activity size={10} className="mb-1" />
                      {t('prov.storage.warning', 'Storage is currently in-memory. Persistence to providers.shared.yaml is restricted by active security policy.')}
                   </div>
                )}
              </div>
            </div>
          </div>
        </Form>
      </GuidedFormDialog>
    </div>
  );
};

function parseTemplates(value: string) {
  return value.split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
    const sep = line.indexOf('=');
    const id = sep === -1 ? line : line.slice(0, sep);
    const vals = sep === -1 ? '' : line.slice(sep + 1);
    return { id: id.trim(), name: id.trim(), description: '', values: parseTemplateValues(vals) };
  }).filter((item) => item.id);
}

function formatTemplates(templates?: ProviderRegistryEntry['templates']): string {
  return (templates || []).map((t) => `${t.id || ''}=${Object.entries(t.values || {}).map(([k, v]) => `${k}=${v}`).join(';')}`).join('\n');
}

function parseTemplateValues(value: string): Record<string, string> {
  return value.split(';').map((item) => item.trim()).filter(Boolean).reduce<Record<string, string>>((r, item) => {
    const [k, v = ''] = item.split('=', 2);
    if (k.trim()) r[k.trim()] = v.trim();
    return r;
  }, {});
}

export default ProvidersPage;

function formatPlatformBinding(providerID?: string, modelID?: string): string {
  const provider = (providerID || '').trim()
  const model = (modelID || '').trim()
  if (!provider && !model) {
    return 'unconfigured'
  }
  if (!provider) {
    return model
  }
  if (!model) {
    return provider
  }
  return `${provider} / ${model}`
}
