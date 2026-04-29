import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'

import { StatusBadge } from '../src/components/ui/status-badge'
import { RawPayloadFold, RiskBadge, StatePanel } from '../src/components/ui/shared-state'

describe('shared state components', () => {
  it('maps queue and execution states to the prototype tone families', () => {
    const openHtml = renderToStaticMarkup(<StatusBadge status="open" />)
    const activeHtml = renderToStaticMarkup(<StatusBadge status="analyzing" />)
    const disabledHtml = renderToStaticMarkup(<StatusBadge status="disabled" />)

    expect(openHtml).toContain('data-tone="warning"')
    expect(activeHtml).toContain('data-tone="active"')
    expect(disabledHtml).toContain('data-tone="muted"')
  })

  it('renders risk badges with explicit risk tones instead of generic status colors', () => {
    const criticalHtml = renderToStaticMarkup(<RiskBadge risk="critical" />)
    const warningHtml = renderToStaticMarkup(<RiskBadge risk="warning" label="Needs review" />)

    expect(criticalHtml).toContain('data-tone="danger"')
    expect(criticalHtml).toContain('critical risk')
    expect(warningHtml).toContain('data-tone="warning"')
    expect(warningHtml).toContain('Needs review')
  })

  it('exposes reusable degraded and disabled state panels', () => {
    const degradedHtml = renderToStaticMarkup(
      <StatePanel title="Provider degraded" description="Assist provider heartbeat is intermittent." tone="degraded" />,
    )
    const disabledHtml = renderToStaticMarkup(
      <StatePanel title="Dangerous action disabled" description="Approval gate is required." tone="disabled" />,
    )

    expect(degradedHtml).toContain('data-tone="degraded"')
    expect(disabledHtml).toContain('data-tone="disabled"')
  })

  it('keeps raw payloads folded by default', () => {
    const foldedHtml = renderToStaticMarkup(
      <RawPayloadFold title="Raw payload" summary="Folded JSON by default">{"{ \"session_id\": \"SES-1048\" }"}</RawPayloadFold>,
    )
    const openHtml = renderToStaticMarkup(
      <RawPayloadFold title="Raw payload" summary="Expanded for debugging" defaultOpen>{'{}'}</RawPayloadFold>,
    )

    expect(foldedHtml).toContain('data-slot="raw-payload-fold"')
    expect(foldedHtml).not.toContain(' open=""')
    expect(openHtml).toContain(' open=""')
  })
})
