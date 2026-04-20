import type { ConnectorManifest } from './api/types';

export const connectorSamples: ConnectorManifest[] = [
  // ─── FIRST-CLASS: SSH ─────────────────────────────────────────────────────
  {
    api_version: 'tars.connector/v1alpha1',
    kind: 'connector',
    metadata: {
      id: 'ssh-main',
      name: 'ssh',
      display_name: 'SSH Connector',
      vendor: 'openssh',
      version: '1.0.0',
      description: 'Direct SSH host command execution. Configure host metadata here and store password/private key in SSH Credential Custody.',
    },
    spec: {
      type: 'execution',
      protocol: 'ssh_native',
      capabilities: [
        { id: 'command.execute', action: 'execute', read_only: false, scopes: ['execution.approved'], description: 'Execute a command on the target host via SSH' },
        { id: 'host.verify', action: 'query', read_only: true, scopes: ['execution.read'], description: 'Verify SSH connectivity to the target host' },
      ],
      connection_form: [
        { key: 'host', label: 'Host', type: 'string', required: true, description: 'Hostname or IP of the SSH server' },
        { key: 'port', label: 'Port', type: 'string', required: false, default: '22', description: 'SSH port (default 22)' },
        { key: 'username', label: 'Username', type: 'string', required: true, description: 'SSH login username' },
        { key: 'credential_id', label: 'Credential ID', type: 'string', required: true, description: 'SSH credential custody ID (secret material is encrypted separately)' },
      ],
      import_export: { exportable: true, importable: true, formats: ['yaml', 'json'] },
    },
    config: {
      values: {
        host: '',
        port: '22',
        username: '',
        credential_id: '',
      },
      secret_refs: {},
    },
    compatibility: {
      tars_major_versions: ['1'],
      upstream_major_versions: ['8', '9'],
      modes: ['managed', 'imported'],
    },
    marketplace: {
      category: 'execution',
      tags: ['ssh', 'execution', 'host', 'command'],
      source: 'official',
    },
  },
  // ─── FIRST-CLASS: VictoriaMetrics ─────────────────────────────────────────
  {
    api_version: 'tars.connector/v1alpha1',
    kind: 'connector',
    metadata: {
      id: 'victoriametrics-main',
      name: 'victoriametrics',
      display_name: 'VictoriaMetrics',
      vendor: 'victoriametrics',
      version: '1.0.0',
      description: 'VictoriaMetrics metrics provider for alert context and PromQL-compatible queries.',
    },
    spec: {
      type: 'metrics',
      protocol: 'victoriametrics_http',
      capabilities: [
        { id: 'query.instant', action: 'query', read_only: true, scopes: ['metrics.read'] },
        { id: 'query.range', action: 'query', read_only: true, scopes: ['metrics.read'] },
      ],
      connection_form: [
        { key: 'base_url', label: 'Base URL', type: 'string', required: true },
        { key: 'bearer_token', label: 'Bearer Token', type: 'secret', required: false, secret: true },
      ],
      import_export: { exportable: true, importable: true, formats: ['yaml', 'json', 'tar.gz'] },
    },
    config: {
      values: {
        base_url: 'http://127.0.0.1:8428',
      },
      secret_refs: {
        bearer_token: '',
      },
    },
    compatibility: {
      tars_major_versions: ['1'],
      upstream_major_versions: ['1'],
      modes: ['managed', 'imported'],
    },
    marketplace: {
      category: 'observability',
      tags: ['metrics', 'victoriametrics', 'promql'],
      source: 'official',
    },
  },
  // ─── FIRST-CLASS: VictoriaLogs ────────────────────────────────────────────
  {
    api_version: 'tars.connector/v1alpha1',
    kind: 'connector',
    metadata: {
      id: 'victorialogs-main',
      name: 'victorialogs',
      display_name: 'VictoriaLogs',
      vendor: 'victoriametrics',
      version: '1.0.0',
      description: 'VictoriaLogs log query connector (LogsQL). Demo uses play-vmlogs.victoriametrics.com — no auth required.',
    },
    spec: {
      type: 'logs',
      protocol: 'victorialogs_http',
      capabilities: [
        { id: 'logs.query', action: 'query', read_only: true, scopes: ['logs.read'], description: 'Query logs using LogsQL expressions' },
        { id: 'victorialogs.query', action: 'query', read_only: true, scopes: ['logs.read'], description: 'Alias for logs.query' },
      ],
      connection_form: [
        { key: 'base_url', label: 'Base URL', type: 'string', required: true, description: 'VictoriaLogs URL, e.g. https://play-vmlogs.victoriametrics.com' },
        { key: 'bearer_token', label: 'Bearer Token', type: 'secret', required: false, secret: true, description: 'Optional bearer token for private instances' },
      ],
      import_export: { exportable: true, importable: true, formats: ['yaml', 'json'] },
    },
    config: {
      values: {
        base_url: 'https://play-vmlogs.victoriametrics.com',
      },
      secret_refs: {
        bearer_token: '',
      },
    },
    compatibility: {
      tars_major_versions: ['1'],
      upstream_major_versions: ['1'],
      modes: ['managed', 'imported'],
    },
    marketplace: {
      category: 'observability',
      tags: ['logs', 'victorialogs', 'logsql'],
      source: 'official',
    },
  },
  // ─── SECONDARY: Prometheus ────────────────────────────────────────────────
  {
    api_version: 'tars.connector/v1alpha1',
    kind: 'connector',
    metadata: {
      id: 'prometheus-main',
      name: 'prometheus',
      display_name: 'Prometheus',
      vendor: 'prometheus',
      version: '1.0.0',
      description: 'Prometheus metrics provider for alert context and trend queries.',
    },
    spec: {
      type: 'metrics',
      protocol: 'prometheus_http',
      capabilities: [
        { id: 'query.instant', action: 'query', read_only: true, scopes: ['metrics.read'] },
        { id: 'query.range', action: 'query', read_only: true, scopes: ['metrics.read'] },
      ],
      connection_form: [
        { key: 'base_url', label: 'Base URL', type: 'string', required: true },
        { key: 'bearer_token', label: 'Bearer Token', type: 'secret', required: false, secret: true },
      ],
      import_export: { exportable: true, importable: true, formats: ['yaml', 'json', 'tar.gz'] },
    },
    config: {
      values: {
        base_url: 'http://127.0.0.1:9090',
      },
      secret_refs: {
        bearer_token: '',
      },
    },
    compatibility: {
      tars_major_versions: ['1'],
      upstream_major_versions: ['2'],
      modes: ['managed', 'imported'],
    },
    marketplace: {
      category: 'observability',
      tags: ['metrics', 'prometheus'],
      source: 'official',
    },
  },
  // ─── SECONDARY: JumpServer ────────────────────────────────────────────────
  {
    api_version: 'tars.connector/v1alpha1',
    kind: 'connector',
    metadata: {
      id: 'jumpserver-main',
      name: 'jumpserver',
      display_name: 'JumpServer',
      vendor: 'jumpserver',
      version: '1.0.0',
      description: 'Managed execution provider through JumpServer bastion host.',
    },
    spec: {
      type: 'execution',
      protocol: 'jumpserver_api',
      capabilities: [
        { id: 'host.read', action: 'query', read_only: true, scopes: ['asset.read'] },
        { id: 'command.execute', action: 'execute', read_only: false, scopes: ['execution.approved'] },
      ],
      connection_form: [
        { key: 'base_url', label: 'Base URL', type: 'string', required: true },
        { key: 'access_key', label: 'Access Key', type: 'secret', required: true, secret: true },
        { key: 'secret_key', label: 'Secret Key', type: 'secret', required: true, secret: true },
      ],
      import_export: { exportable: true, importable: true, formats: ['yaml', 'json', 'tar.gz'] },
    },
    config: {
      values: {
        base_url: 'https://jumpserver.example.com',
      },
      secret_refs: {
        access_key: '',
        secret_key: '',
      },
    },
    compatibility: {
      tars_major_versions: ['1'],
      upstream_major_versions: ['3'],
      modes: ['managed', 'imported'],
    },
    marketplace: {
      category: 'execution',
      tags: ['jumpserver', 'approval'],
      source: 'official',
    },
  },
];


function createDraftConfigFromTemplate(template: ConnectorManifest): ConnectorManifest['config'] {
  const values: Record<string, string> = {};
  const secretRefs: Record<string, string> = {};

  for (const field of template.spec.connection_form || []) {
    const key = field.key?.trim();
    if (!key) {
      continue;
    }
    if (field.secret) {
      secretRefs[key] = '';
      continue;
    }
    values[key] = field.default || '';
  }

  return { values, secret_refs: secretRefs };
}

export function connectorTemplateMatches(template: ConnectorManifest, manifest: ConnectorManifest): boolean {
  return Boolean(
    template.spec.protocol &&
      template.spec.protocol === manifest.spec.protocol &&
      template.spec.type === manifest.spec.type,
  );
}

export function findConnectorSampleByID(sampleID: string): ConnectorManifest | undefined {
  return connectorSamples.find((sample) => sample.metadata.id === sampleID);
}

export function findConnectorSampleByProtocol(protocol: string): ConnectorManifest | undefined {
  return connectorSamples.find((sample) => sample.spec.protocol === protocol);
}

export function createConnectorDraftFromTemplate(
  template: ConnectorManifest,
  current?: ConnectorManifest,
): ConnectorManifest {
  const seed = current || createEmptyConnectorManifest();
  return {
    ...seed,
    metadata: {
      ...seed.metadata,
      id: seed.metadata.id || '',
      name: seed.metadata.name || '',
      display_name: seed.metadata.display_name || template.metadata.display_name || '',
      vendor: template.metadata.vendor || seed.metadata.vendor,
      version: template.metadata.version || seed.metadata.version,
      description: seed.metadata.description || template.metadata.description || '',
    },
    spec: {
      type: template.spec.type,
      protocol: template.spec.protocol,
      capabilities: template.spec.capabilities || [],
      connection_form: template.spec.connection_form || [],
      import_export: template.spec.import_export,
    },
    config: createDraftConfigFromTemplate(template),
    compatibility: template.compatibility,
    marketplace: template.marketplace,
    enabled: seed.enabled ?? true,
  };
}

export function applyConnectorProtocolPreset(manifest: ConnectorManifest, protocol: string): ConnectorManifest {
  const preset = findConnectorSampleByProtocol(protocol);
  if (!preset) {
    return {
      ...manifest,
      spec: {
        ...manifest.spec,
        protocol,
      },
    };
  }
  return {
    ...createConnectorDraftFromTemplate(preset, manifest),
    metadata: {
      ...createConnectorDraftFromTemplate(preset, manifest).metadata,
      id: manifest.metadata.id,
      name: manifest.metadata.name,
      display_name: manifest.metadata.display_name || preset.metadata.display_name || '',
      description: manifest.metadata.description || preset.metadata.description || '',
    },
  };
}

export function createEmptyConnectorManifest(): ConnectorManifest {
  return {
    api_version: 'tars.connector/v1alpha1',
    kind: 'connector',
    enabled: true,
    metadata: {
      id: '',
      name: '',
      display_name: '',
      vendor: '',
      version: '1.0.0',
      description: '',
    },
    spec: {
      type: '',
      protocol: '',
      capabilities: [],
      connection_form: [],
      import_export: { exportable: true, importable: true, formats: ['yaml', 'json'] },
    },
    config: { values: {}, secret_refs: {} },
    compatibility: { tars_major_versions: ['1'], upstream_major_versions: [], modes: ['managed'] },
    marketplace: { category: '', tags: [], source: 'custom' },
  };
}

export function normalizeConnectorManifest(manifest: ConnectorManifest): ConnectorManifest {
  const secretKeys = new Set(
    (manifest.spec.connection_form || []).filter((field) => field.secret && field.key).map((field) => field.key as string),
  );
  const plainValues = { ...(manifest.config?.values || {}) };
  const secretRefs = { ...(manifest.config?.secret_refs || {}) };

  for (const key of Object.keys(plainValues)) {
    if (secretKeys.has(key)) {
      delete plainValues[key];
      if (!(key in secretRefs)) {
        secretRefs[key] = '';
      }
    }
  }

  for (const key of Object.keys(secretRefs)) {
    if (!secretKeys.has(key)) {
      delete secretRefs[key];
    }
  }

  return {
    ...manifest,
    lifecycle: undefined,
    metadata: {
      ...manifest.metadata,
      name: manifest.metadata.name || manifest.metadata.id,
    },
    config: {
      values: plainValues,
      secret_refs: secretRefs,
    },
  };
}
