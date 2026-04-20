type Translate = (key: string, params?: Record<string, unknown> | string) => string

export const providerDiscoveryMessages = {
  discovered: (t: Translate, count: number) => t('prov.discovery.discovered', { count }),
  unreachable: (t: Translate, model: string) => t('prov.discovery.unreachable', { model }),
}
