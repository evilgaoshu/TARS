import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

function readFixture(relativePath: string) {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), 'utf8')
}

const providersPage = readFixture('../src/pages/providers/ProvidersPage.tsx')
const agentRolesPage = readFixture('../src/pages/identity/AgentRolesPage.tsx')

describe('provider page localization cleanup', () => {
  it('does not keep the known user-facing hardcoded strings', () => {
    expect(providersPage).not.toContain("'Provider Name/ID is required'")
    expect(providersPage).not.toContain("'Vendor is required'")
    expect(providersPage).not.toContain("'Protocol is required'")
    expect(providersPage).not.toContain("'Must be a valid URL'")
    expect(providersPage).not.toContain('> Authentication<')
    expect(providersPage).not.toContain('placeholder="default=temperature=0.7;max_tokens=2048"')
  })
})

describe('agent role page localization cleanup', () => {
  it('does not keep the known user-facing hardcoded placeholders and fallbacks', () => {
    expect(agentRolesPage).not.toContain('placeholder="e.g. diagnosis"')
    expect(agentRolesPage).not.toContain('placeholder="e.g. Diagnosis Agent"')
    expect(agentRolesPage).not.toContain('placeholder="What this role does"')
    expect(agentRolesPage).not.toContain('placeholder="Agent system prompt..."')
    expect(agentRolesPage).not.toContain('placeholder="cautious, read-only, sre"')
    expect(agentRolesPage).not.toContain('placeholder="skill_a, skill_b"')
    expect(agentRolesPage).not.toContain('placeholder="rm -rf, shutdown"')
    expect(agentRolesPage).not.toContain("|| '(none)'")
  })
})
