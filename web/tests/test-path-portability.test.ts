import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

function readTestFile(relativePath: string) {
  return readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), 'utf8')
}

describe('test file path portability', () => {
  it('does not hardcode a local workspace path in locale cleanup test', () => {
    const source = readTestFile('./locale-cleanup.test.ts')

    expect(source).not.toContain('/Users/yue/TARS/')
  })

  it('does not hardcode a local workspace path in UI governance test', () => {
    const source = readTestFile('./ui-governance.test.ts')

    expect(source).not.toContain('/Users/yue/TARS/')
  })
})
