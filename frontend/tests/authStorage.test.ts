import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

test('authentication data is not persisted in browser storage', async () => {
  const storageKeys = new Set<string>();
  for (const sourcePath of ['src/App.tsx', 'src/services/api.ts']) {
    const source = await readFile(new URL(`../${sourcePath}`, import.meta.url), 'utf8');
    const storageKeyPattern = /(?:localStorage|sessionStorage)\.(?:getItem|setItem)\(\s*(['"])([^'"]+)\1/g;
    for (const match of source.matchAll(storageKeyPattern)) {
      storageKeys.add(match[2]!);
    }
  }

  assert.deepEqual([...storageKeys], ['secure-switch-theme']);
});
