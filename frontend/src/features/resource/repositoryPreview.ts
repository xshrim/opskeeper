export type RepositoryPreviewEntry = {
  path: string;
  type: string;
  size?: number;
};

export type RepositoryPreviewNode = {
  name: string;
  path: string;
  type: string;
  size?: number;
  children: RepositoryPreviewNode[];
  depth: number;
};

export function buildRepositoryPreviewTree(entries: RepositoryPreviewEntry[]) {
  const roots: RepositoryPreviewNode[] = [];
  const childMaps = new Map<
    RepositoryPreviewNode[],
    Map<string, RepositoryPreviewNode>
  >([[roots, new Map()]]);

  for (const entry of entries) {
    const parts = entry.path.split('/').filter(Boolean);
    let children = roots;
    for (const [index, name] of parts.entries()) {
      let siblings = childMaps.get(children);
      if (!siblings) childMaps.set(children, (siblings = new Map()));
      let node = siblings.get(name);
      const path = parts.slice(0, index + 1).join('/');
      if (!node) {
        node = {
          name,
          path,
          type: index === parts.length - 1 ? entry.type : 'directory',
          size: index === parts.length - 1 ? entry.size : undefined,
          children: [],
          depth: index
        };
        siblings.set(name, node);
        children.push(node);
      }
      children = node.children;
    }
  }

  return roots;
}

export function visibleRepositoryPreviewNodes(
  nodes: RepositoryPreviewNode[],
  expandedPaths: Set<string>
): RepositoryPreviewNode[] {
  return nodes.flatMap((node) => [
    node,
    ...(expandedPaths.has(node.path)
      ? visibleRepositoryPreviewNodes(node.children, expandedPaths)
      : [])
  ]);
}
