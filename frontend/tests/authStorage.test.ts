import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

test('authentication data is not persisted in localStorage', async () => {
  for (const sourcePath of ['src/App.tsx', 'src/services/api.ts']) {
    const source = await readFile(new URL(`../${sourcePath}`, import.meta.url), 'utf8');
    assert.equal(source.includes('localStorage'), false, `${sourcePath} persists authentication data in localStorage`);
  }
});
