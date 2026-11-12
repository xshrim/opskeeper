import { describe, expect, it } from 'vitest';
import {
  buildRepositoryPreviewTree,
  visibleRepositoryPreviewNodes
} from './repositoryPreview';

describe('repository preview tree', () => {
  it('builds nested directories and exposes children only when expanded', () => {
    const tree = buildRepositoryPreviewTree([
      { path: 'src/main.ts', type: 'file', size: 10 },
      { path: 'src/lib/util.ts', type: 'file', size: 20 },
      { path: 'README.md', type: 'file', size: 5 }
    ]);

    expect(tree.map(({ path }) => path)).toEqual(['src', 'README.md']);
    expect(tree[0].children.map(({ path }) => path)).toEqual([
      'src/main.ts',
      'src/lib'
    ]);
    expect(
      visibleRepositoryPreviewNodes(tree, new Set()).map(({ path }) => path)
    ).toEqual(['src', 'README.md']);
    expect(
      visibleRepositoryPreviewNodes(tree, new Set(['src'])).map(
        ({ path }) => path
      )
    ).toEqual(['src', 'src/main.ts', 'src/lib', 'README.md']);
  });
});
